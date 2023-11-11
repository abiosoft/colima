package lima

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/core"
	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/inotify"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/abiosoft/colima/util/yamlutil"
	"github.com/sirupsen/logrus"
)

// New creates a new virtual machine.
func New(host environment.HostActions) environment.VM {
	// environment variables for the subprocesses
	var envs []string
	envLimaInstance := limaInstanceEnvVar + "=" + config.CurrentProfile().ID
	envSubprocess := config.SubprocessProfileEnvVar + "=" + config.CurrentProfile().ShortName
	envBinary := osutil.EnvColimaBinary + "=" + osutil.Executable()
	envs = append(envs, envLimaInstance, envSubprocess, envBinary)

	home := limautil.LimaHome()
	// consider making this truly flexible to support other VMs
	return &limaVM{
		host:         host.WithEnv(envs...),
		home:         home,
		CommandChain: cli.New("vm"),
		daemon:       daemon.NewManager(host),
	}
}

const (
	limaInstanceEnvVar = "LIMA_INSTANCE"
	lima               = "lima"
	limactl            = "limactl"
)

func (l limaVM) limaConfFile() string {
	return filepath.Join(l.home, config.CurrentProfile().ID, "lima.yaml")
}

var _ environment.VM = (*limaVM)(nil)

type limaVM struct {
	host environment.HostActions
	cli.CommandChain

	// keep config in case of restart
	conf config.Config

	// lima config
	limaConf Config

	// lima config directory
	home string

	// network between host and the vm
	daemon daemon.Manager
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

func (l *limaVM) startDaemon(ctx context.Context, conf config.Config) (context.Context, error) {
	isQEMU := conf.VMType == QEMU
	isVZ := conf.VMType == VZ

	// limited to macOS (with Qemu driver)
	// or vz with inotify enabled
	if !util.MacOS() || (isVZ && !conf.MountINotify) {
		return ctx, nil
	}

	ctxKeyVmnet := daemon.CtxKey(vmnet.Name)
	ctxKeyInotify := daemon.CtxKey(inotify.Name)

	// use a nested chain for convenience
	a := l.Init(ctx)
	log := l.Logger(ctx)

	networkInstalledKey := struct{ key string }{key: "network_installed"}

	// add inotify to daemon
	if conf.MountINotify {
		a.Add(func() error {
			ctx = context.WithValue(ctx, ctxKeyInotify, true)
			deps, _ := l.daemon.Dependencies(ctx, conf)
			if err := deps.Install(l.host); err != nil {
				return fmt.Errorf("error setting up inotify dependencies: %w", err)
			}
			return nil
		})
	}

	// add network processes to daemon
	if isQEMU {
		a.Stage("preparing network")
		a.Add(func() error {
			if conf.Network.Address {
				ctx = context.WithValue(ctx, ctxKeyVmnet, true)
			}
			deps, root := l.daemon.Dependencies(ctx, conf)
			if deps.Installed() {
				ctx = context.WithValue(ctx, networkInstalledKey, true)
				return nil
			}

			// if user interaction is not required (i.e. root),
			// no need for another verbose info.
			if root {
				log.Println("dependencies missing for setting up reachable IP address")
				log.Println("sudo password may be required")
			}

			// install deps
			err := deps.Install(l.host)
			if err != nil {
				ctx = context.WithValue(ctx, networkInstalledKey, false)
			}
			return err
		})
	}

	// start daemon
	a.Add(func() error {
		return l.daemon.Start(ctx, conf)
	})

	statusKey := struct{ key string }{key: "daemonStatus"}
	// delay to ensure that the processes have started
	if conf.Network.Address || conf.MountINotify {
		a.Retry("", time.Second*1, 15, func(i int) error {
			s, err := l.daemon.Running(ctx, conf)
			ctx = context.WithValue(ctx, statusKey, s)
			if err != nil {
				return err
			}
			if !s.Running {
				return fmt.Errorf("daemon is not running")
			}
			for _, p := range s.Processes {
				if !p.Running {
					return p.Error
				}
			}
			return nil
		})
	}

	// network failure is not fatal
	if err := a.Exec(); err != nil {
		if isQEMU {
			func() {
				installed, _ := ctx.Value(networkInstalledKey).(bool)
				if !installed {
					log.Warnln(fmt.Errorf("error setting up network dependencies: %w", err))
					return
				}

				status, ok := ctx.Value(statusKey).(daemon.Status)
				if !ok {
					return
				}
				if !status.Running {
					log.Warnln(fmt.Errorf("error starting network: %w", err))
					return
				}

				for _, p := range status.Processes {
					// TODO: handle inotify separate from network
					if p.Name == inotify.Name {
						continue
					}
					if !p.Running {
						ctx = context.WithValue(ctx, daemon.CtxKey(p.Name), false)
						log.Warnln(fmt.Errorf("error starting %s: %w", p.Name, err))
					}
				}
			}()
		}
	}

	// check if inotify is running
	if conf.MountINotify {
		if inotifyEnabled, _ := ctx.Value(ctxKeyInotify).(bool); !inotifyEnabled {
			log.Warnln("error occurred enabling inotify daemon")
		}
	}

	// preserve vmnet context
	if vmnetEnabled, _ := ctx.Value(ctxKeyVmnet).(bool); vmnetEnabled {
		// env var for subprocess to detect vmnet
		l.host = l.host.WithEnv(vmnet.SubProcessEnvVar + "=1")
	}

	return ctx, nil
}

func (l *limaVM) Start(ctx context.Context, conf config.Config) error {
	a := l.Init(ctx)

	if l.Created() {
		return l.resume(ctx, conf)
	}

	a.Add(func() (err error) {
		ctx, err = l.startDaemon(ctx, conf)
		return err
	})

	a.Stage("creating and starting")
	configFile := filepath.Join(os.TempDir(), config.CurrentProfile().ID+".yaml")

	a.Add(func() (err error) {
		l.limaConf, err = newConf(ctx, conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(l.limaConf, configFile)
	})
	a.Add(func() error {
		return l.host.Run(limactl, "start", "--tty=false", configFile)
	})
	a.Add(func() error {
		return os.Remove(configFile)
	})

	l.addPostStartActions(a, conf)

	// adding it to command chain to execute only after successful startup.
	a.Add(func() error {
		l.conf = conf
		return nil
	})

	return a.Exec()
}

func (l *limaVM) resume(ctx context.Context, conf config.Config) error {
	log := l.Logger(ctx)
	a := l.Init(ctx)

	if l.Running(ctx) {
		log.Println("already running")
		return nil
	}

	a.Add(func() (err error) {
		ctx, err = l.startDaemon(ctx, conf)
		return err
	})

	a.Add(func() (err error) {
		// disk must be resized before starting
		conf = l.syncDiskSize(ctx, conf)

		l.limaConf, err = newConf(ctx, conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(l.limaConf, l.limaConfFile())
	})

	a.Stage("starting")
	a.Add(func() error {
		return l.host.Run(limactl, "start", config.CurrentProfile().ID)
	})

	l.addPostStartActions(a, conf)

	return a.Exec()
}

func (l limaVM) Running(_ context.Context) bool {
	i, err := limautil.Instance()
	if err != nil {
		logrus.Trace(fmt.Errorf("error retrieving running instance: %w", err))
		return false
	}
	return i.Running()
}

func (l limaVM) Stop(ctx context.Context, force bool) error {
	log := l.Logger(ctx)
	a := l.Init(ctx)
	if !l.Running(ctx) && !force {
		log.Println("not running")
		return nil
	}

	a.Stage("stopping")

	if util.MacOS() {
		conf, _ := limautil.InstanceConfig()
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			err := l.daemon.Stop(ctx, conf)
			if err != nil {
				err = cli.ErrNonFatal(err)
			}
			return err
		})
	}

	a.Add(func() error {
		if force {
			return l.host.Run(limactl, "stop", "--force", config.CurrentProfile().ID)
		}
		return l.host.Run(limactl, "stop", config.CurrentProfile().ID)
	})

	return a.Exec()
}

func (l limaVM) Teardown(ctx context.Context) error {
	a := l.Init(ctx)

	if util.MacOS() {
		conf, _ := limautil.InstanceConfig()
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			return l.daemon.Stop(ctx, conf)
		})
	}

	a.Add(func() error {
		return l.host.Run(limactl, "delete", "--force", config.CurrentProfile().ID)
	})

	return a.Exec()
}

func (l limaVM) Restart(ctx context.Context) error {
	if l.conf.Empty() {
		return fmt.Errorf("cannot restart, VM not previously started")
	}

	if err := l.Stop(ctx, false); err != nil {
		return err
	}

	// minor delay to prevent possible race condition.
	time.Sleep(time.Second * 2)

	if err := l.Start(ctx, l.conf); err != nil {
		return err
	}

	return nil
}

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.Run(args...)
	})

	return a.Exec()
}

func (l limaVM) SSH(workingDir string, args ...string) error {
	args = append([]string{limactl, "shell", "--workdir", workingDir, config.CurrentProfile().ID}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return a.Exec()
}

func (l limaVM) RunInteractive(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return a.Exec()
}

func (l limaVM) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunWith(stdin, stdout, args...)
	})

	return a.Exec()
}

func (l limaVM) RunOutput(args ...string) (out string, err error) {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() (err error) {
		out, err = l.host.RunOutput(args...)
		return
	})

	err = a.Exec()
	return
}

func (l limaVM) RunQuiet(args ...string) (err error) {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() (err error) {
		return l.host.RunQuiet(args...)
	})

	err = a.Exec()
	return
}

func (l limaVM) Host() environment.HostActions {
	return l.host
}

func (l limaVM) Env(s string) (string, error) {
	ctx := context.Background()
	if !l.Running(ctx) {
		return "", fmt.Errorf("not running")
	}
	return l.RunOutput("echo", "$"+s)
}

func (l limaVM) Created() bool {
	stat, err := os.Stat(l.limaConfFile())
	return err == nil && !stat.IsDir()
}

const configFile = "/etc/colima/colima.json"

func (l limaVM) getConf() map[string]string {
	obj := map[string]string{}
	b, err := l.Read(configFile)
	if err != nil {
		return obj
	}

	// we do not care if it fails
	_ = json.Unmarshal([]byte(b), &obj)

	return obj
}
func (l limaVM) Get(key string) string {
	if val, ok := l.getConf()[key]; ok {
		return val
	}

	return ""
}

func (l limaVM) Set(key, value string) error {
	obj := l.getConf()
	obj[key] = value

	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshalling settings to json: %w", err)
	}

	if err := l.Run("sudo", "mkdir", "-p", filepath.Dir(configFile)); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	if err := l.Write(configFile, b); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}

func (l limaVM) User() (string, error) {
	return l.RunOutput("whoami")
}

func (l limaVM) Arch() environment.Arch {
	a, _ := l.RunOutput("uname", "-m")
	return environment.Arch(a)
}

func includesHost(hostsFileContent, host string, ip net.IP) bool {
	scanner := bufio.NewScanner(strings.NewReader(hostsFileContent))
	for scanner.Scan() {
		str := strings.Fields(scanner.Text())
		if len(str) == 0 || str[0] != ip.String() {
			continue
		}
		if len(str) > 1 && str[1] == host {
			return true
		}
	}
	return false
}

func (l *limaVM) syncDiskSize(ctx context.Context, conf config.Config) config.Config {
	log := l.Logger(ctx)
	instance, err := limautil.InstanceConfig()
	if err != nil {
		// instance config missing, ignore
		return conf
	}

	resized := func() bool {
		if instance.Disk == conf.Disk {
			// nothing to do
			return false
		}

		if conf.VMType == VZ {
			log.Warnln("dynamic disk resize not supported for VZ driver, ignoring...")
			return false
		}

		size := conf.Disk - instance.Disk
		if size < 0 {
			log.Warnln("disk size cannot be reduced, ignoring...")
			return false
		}

		sizeStr := fmt.Sprintf("%dG", conf.Disk)
		args := []string{"qemu-img", "resize"}
		disk := limautil.ColimaDiffDisk(config.CurrentProfile().ID)
		args = append(args, disk, sizeStr)

		// qemu-img resize /path/to/diffdisk +10G
		if err := l.host.RunQuiet(args...); err != nil {
			log.Warnln(fmt.Errorf("unable to resize disk: %w", err))
			return false
		}

		log.Printf("resizing disk to %dGiB...", conf.Disk)
		return true
	}()

	if !resized {
		conf.Disk = instance.Disk
	}

	return conf
}

func (l *limaVM) addPostStartActions(a *cli.ActiveCommandChain, conf config.Config) {
	// package dependencies
	a.Add(func() error {
		return l.installDependencies(a.Logger(), conf)
	})

	// containerd dependencies
	if conf.Runtime == containerd.Name {
		a.Add(func() error {
			return core.SetupContainerdUtils(l.host, l, environment.Arch(conf.Arch))
		})
	}

	// binfmt
	a.Add(func() error {
		return core.SetupBinfmt(l.host, l, environment.Arch(conf.Arch))
	})

	// registry certs
	a.Add(l.copyCerts)

	// use rosetta for x86_64 emulation
	a.Add(func() error {
		if !l.limaConf.Rosetta.Enabled {
			return nil
		}

		// enable rosetta
		err := l.Run("sudo", "sh", "-c", `stat /proc/sys/fs/binfmt_misc/rosetta || echo ':rosetta:M::\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00:\xff\xff\xff\xff\xff\xfe\xfe\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/mnt/lima-rosetta/rosetta:OCF' > /proc/sys/fs/binfmt_misc/register`)
		if err != nil {
			logrus.Warn(fmt.Errorf("unable to enable rosetta: %w", err))
			return nil
		}

		// disable qemu
		if err := l.RunQuiet("stat", "/proc/sys/fs/binfmt_misc/qemu-x86_64"); err == nil {
			err = l.Run("sudo", "sh", "-c", `echo 0 > /proc/sys/fs/binfmt_misc/qemu-x86_64`)
			if err != nil {
				logrus.Warn(fmt.Errorf("unable to disable qemu x86_84 emulation: %w", err))
			}
		}

		return nil
	})

	// preserve state
	a.Add(func() error {
		if err := configmanager.SaveToFile(conf, limautil.ColimaStateFile(config.CurrentProfile().ID)); err != nil {
			logrus.Warnln(fmt.Errorf("error persisting Colima state: %w", err))
		}
		return nil
	})
}

var dependencyPackages = []string{
	// docker
	"docker.io",
	// utilities
	"htop", "vim",
}

// cacheDependencies downloads the ubuntu deb files to a path on the host.
// The return value is the directory of the downloaded deb files.
func (l *limaVM) cacheDependencies(log *logrus.Entry, conf config.Config) (string, error) {
	codename, err := l.RunOutput("sh", "-c", `grep "^UBUNTU_CODENAME" /etc/os-release | cut -d= -f2`)
	if err != nil {
		return "", fmt.Errorf("error retrieving OS version from vm: %w", err)
	}

	arch := environment.Arch(conf.Arch).Value()
	dir := filepath.Join(config.CacheDir(), "packages", codename, string(arch))
	if err := fsutil.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating cache directory for OS packages: %w", err)
	}

	doneFile := filepath.Join(dir, ".downloaded")
	if _, err := os.Stat(doneFile); err == nil {
		// already downloaded
		return dir, nil
	}

	output := ""
	for _, p := range dependencyPackages {
		line := fmt.Sprintf(`sudo apt-get install --reinstall --print-uris -qq "%s" | cut -d"'" -f2`, p)
		out, err := l.RunOutput("sh", "-c", line)
		if err != nil {
			return "", fmt.Errorf("error fetching dependencies list: %w", err)
		}
		output += out + " "
	}

	debPackages := strings.Fields(output)

	// progress bar for Ubuntu deb packages download.
	// TODO: extract this into re-usable progress bar for multi-downloads
	for i, p := range debPackages {
		// status feedback
		log.Infof("downloading package %d of %d ...", i+1, len(debPackages))

		// download
		if err := l.host.RunInteractive(
			"sh", "-c",
			fmt.Sprintf(`cd %s && curl -LO -# %s`, dir, p),
		); err != nil {
			return "", fmt.Errorf("error downloading dependency: %w", err)
		}

		// clear terminal
		terminal.ClearLine() // for curl output
		terminal.ClearLine() // for log message
	}

	// write a file to signify it is done
	return dir, l.host.RunQuiet("touch", doneFile)
}

func (l *limaVM) installDependencies(log *logrus.Entry, conf config.Config) error {
	// cache dependencies
	dir, err := l.cacheDependencies(log, conf)
	if err != nil {
		log.Warnln("error caching dependencies: %w", err)
		log.Warnln("falling back to normal package install", err)
		return l.Run("sudo apt install -y " + strings.Join(dependencyPackages, " "))
	}

	// validate if packages were previously installed
	installed := true
	for _, p := range dependencyPackages {
		if err := l.RunQuiet("dpkg", "-s", p); err != nil {
			installed = false
			break
		}
	}

	if installed {
		return nil
	}

	// install packages
	return l.Run("sh", "-c", "sudo dpkg -i "+dir+"/*.deb")
}

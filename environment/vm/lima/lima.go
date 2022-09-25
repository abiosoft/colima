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
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/gvproxy"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/qemu"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
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
	// limited to macOS for now
	if !util.MacOS() {
		return ctx, nil
	}

	ctxKeyVmnet := daemon.CtxKey(vmnet.Name)
	ctxKeyGVProxy := daemon.CtxKey(gvproxy.Name)

	// use a nested chain for convenience
	a := l.Init(ctx)
	log := l.Logger(ctx)

	installedKey := struct{ key string }{key: "installed"}

	a.Stage("preparing network")
	a.Add(func() error {
		if conf.Network.Driver == gvproxy.Name {
			ctx = context.WithValue(ctx, ctxKeyGVProxy, true)
		}
		if conf.Network.Address {
			ctx = context.WithValue(ctx, ctxKeyVmnet, true)
		}
		deps, root := l.daemon.Dependencies(ctx)
		if deps.Installed() {
			ctx = context.WithValue(ctx, installedKey, true)
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
			ctx = context.WithValue(ctx, installedKey, false)
		}
		return err
	})

	a.Add(func() error {
		return l.daemon.Start(ctx)
	})

	// delay to ensure that the vmnet is running
	statusKey := struct{ key string }{key: "networkStatus"}
	if conf.Network.Address {
		a.Retry("", time.Second*3, 5, func(i int) error {
			s, err := l.daemon.Running(ctx)
			ctx = context.WithValue(ctx, statusKey, s)
			if err != nil {
				return err
			}
			if !s.Running {
				return fmt.Errorf("network process is not running")
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
		func() {
			installed, _ := ctx.Value(installedKey).(bool)
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
				if !p.Running {
					ctx = context.WithValue(ctx, daemon.CtxKey(p.Name), false)
					log.Warnln(fmt.Errorf("error starting %s: %w", p.Name, err))
				}
			}
		}()
	}

	// preserve gvproxy context
	if gvproxyEnabled, _ := ctx.Value(daemon.CtxKey(gvproxy.Name)).(bool); gvproxyEnabled {
		var envs []string

		// env var for subproxy to detect gvproxy
		envs = append(envs, gvproxy.SubProcessEnvVar+"=1")
		// use qemu wrapper for Lima by specifying wrapper binaries via env var
		envs = append(envs, qemu.LimaDir().BinsEnvVar()...)

		l.host = l.host.WithEnv(envs...)
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

	a.Add(func() error {
		limaConf, err := newConf(ctx, conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(limaConf, configFile)
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

func (l limaVM) resume(ctx context.Context, conf config.Config) error {
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

	a.Add(func() error {
		limaConf, err := newConf(ctx, conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(limaConf, l.limaConfFile())
	})

	a.Stage("starting")
	a.Add(func() error {
		return l.host.Run(limactl, "start", config.CurrentProfile().ID)
	})

	l.addPostStartActions(a, conf)

	return a.Exec()
}

func (l limaVM) Running(ctx context.Context) bool {
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
	if !l.Running(ctx) {
		log.Println("not running")
		return nil
	}

	a.Stage("stopping")

	if util.MacOS() {
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			return l.daemon.Stop(ctx)
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
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			return l.daemon.Stop(ctx)
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

	if err := l.Write(configFile, string(b)); err != nil {
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

func (l limaVM) addHost(host string, ip net.IP) error {
	if hostsFile, err := l.Read("/etc/hosts"); err == nil && includesHost(hostsFile, host, ip) {
		return nil
	} else if err != nil {
		logrus.Warnln(fmt.Errorf("cannot read /etc/hosts in the VM: %w", err))
	}

	line := fmt.Sprintf("%s\t%s", ip.String(), host)
	line = fmt.Sprintf("echo -e %s >> /etc/hosts", strconv.Quote(line))
	return l.Run("sudo", "sh", "-c", line)
}

func includesHost(hostsFileContent, host string, ip net.IP) bool {
	scanner := bufio.NewScanner(strings.NewReader(hostsFileContent))
	for scanner.Scan() {
		str := strings.Fields(scanner.Text())
		if str[0] != ip.String() {
			continue
		}
		if len(str) > 1 && str[1] == host {
			return true
		}
	}
	return false
}

func (l limaVM) addPostStartActions(a *cli.ActiveCommandChain, conf config.Config) {
	// host file
	{
		// add docker host alias
		a.Add(func() error {
			return l.addHost("host.docker.internal", net.ParseIP("192.168.5.2"))
		})
		// prevent chroot host error for layer
		a.Add(func() error {
			return l.addHost(config.CurrentProfile().ID, net.ParseIP("127.0.0.1"))
		})
	}

	// registry certs
	a.Add(l.copyCerts)

	// preserve state
	a.Add(func() error {
		if err := configmanager.SaveToFile(conf, limautil.ColimaStateFile(config.CurrentProfile().ID)); err != nil {
			logrus.Warnln(fmt.Errorf("error persisting Colima state: %w", err))
		}
		return nil
	})
}

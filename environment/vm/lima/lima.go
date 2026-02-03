package lima

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/core"
	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/store"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/abiosoft/colima/util/yamlutil"
	"github.com/sirupsen/logrus"
)

func init() {
	vm.RegisterVM(vm.BackendLima, New)
	vm.RegisterInstanceLister(vm.BackendLima, &limaInstanceLister{})
}

// limaInstanceLister implements vm.InstanceLister for Lima.
type limaInstanceLister struct{}

// Instances returns all Lima instances as vm.InstanceInfo.
func (l *limaInstanceLister) Instances(ids ...string) ([]vm.InstanceInfo, error) {
	limaInstances, err := limautil.Instances(ids...)
	if err != nil {
		return nil, err
	}

	instances := make([]vm.InstanceInfo, len(limaInstances))
	for i, inst := range limaInstances {
		instances[i] = vm.InstanceInfo{
			Name:      inst.Name,
			Status:    inst.Status,
			Arch:      inst.Arch,
			CPU:       inst.CPU,
			Memory:    inst.Memory,
			Disk:      inst.Disk,
			Dir:       inst.Dir,
			IPAddress: inst.IPAddress,
			Runtime:   inst.Runtime,
			Backend:   string(vm.BackendLima),
		}
	}
	return instances, nil
}

// New creates a new virtual machine.
func New(host environment.HostActions) environment.VM {
	// lima config directory
	limaHome := config.LimaDir()

	// environment variables for the subprocesses
	var envs []string
	envHome := limautil.EnvLimaHome + "=" + limaHome
	envLimaInstance := envLimaInstance + "=" + config.CurrentProfile().ID
	envBinary := osutil.EnvColimaBinary + "=" + osutil.Executable()
	envs = append(envs, envHome, envLimaInstance, envBinary)

	// consider making this truly flexible to support other VMs
	return &limaVM{
		host:         host.WithEnv(envs...),
		limaHome:     limaHome,
		CommandChain: cli.New("vm"),
		daemon:       daemon.NewManager(host),
	}
}

const (
	envLimaInstance = "LIMA_INSTANCE"
	lima            = "lima"
	limactl         = limautil.LimactlCommand
)

var _ environment.VM = (*limaVM)(nil)

type limaVM struct {
	host environment.HostActions
	cli.CommandChain

	// keep config in case of restart
	conf config.Config

	// lima config
	limaConf limaconfig.Config

	// lima config directory
	limaHome string

	// network between host and the vm
	daemon daemon.Manager
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

func (l *limaVM) Start(ctx context.Context, conf config.Config) error {
	a := l.Init(ctx)

	l.prepareHost(conf)

	if l.Created() {
		return l.resume(ctx, conf)
	}

	a.Add(func() (err error) {
		ctx, err = l.startDaemon(ctx, conf)
		return err
	})

	a.Stage("creating and starting")
	confFile := filepath.Join(os.TempDir(), config.CurrentProfile().ID+".yaml")

	a.Add(func() (err error) {
		l.limaConf, err = newConf(ctx, conf)
		return err
	})

	a.Add(l.assertQemu)

	a.Add(func() error {
		return l.createRuntimeDisk(conf)
	})

	a.Add(func() error {
		return l.downloadDiskImage(ctx, conf)
	})

	a.Add(func() error {
		return yamlutil.WriteYAML(l.limaConf, confFile)
	})

	a.Add(func() error { return l.writeNetworkFile(conf) })

	a.Add(func() error {
		return l.host.Run(limactl, "start", "--tty=false", confFile)
	})
	a.Add(func() error {
		return os.Remove(confFile)
	})

	// adding it to command chain to execute only after successful startup.
	a.Add(func() error {
		l.conf = conf
		return nil
	})

	l.addPostStartActions(a, conf)

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
		return err
	})

	a.Add(l.assertQemu)

	a.Add(func() error {
		l.useRuntimeDisk(conf)
		return nil
	})

	a.Add(l.setDiskImage)

	a.Add(func() error {
		err := yamlutil.WriteYAML(l.limaConf, config.CurrentProfile().LimaFile())
		return err
	})

	a.Add(func() error { return l.writeNetworkFile(conf) })

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
		conf, _ := configmanager.LoadInstance()
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			err := l.daemon.Stop(ctx, conf)
			if err != nil {
				err = cli.ErrNonFatal(err)
			}
			return err
		})
	}

	a.Add(func() error { l.removeHostAddresses(); return nil })

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
		conf, _ := configmanager.LoadInstance()
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
	stat, err := os.Stat(config.CurrentProfile().LimaFile())
	return err == nil && !stat.IsDir()
}

func (l limaVM) User() (string, error) {
	return l.RunOutput("whoami")
}

func (l limaVM) Arch() environment.Arch {
	a, _ := l.RunOutput("uname", "-m")
	return environment.Arch(a)
}

func (l *limaVM) addPostStartActions(a *cli.ActiveCommandChain, conf config.Config) {
	// setup dns
	a.Add(func() error {
		if err := l.setupDNS(conf); err != nil {
			return fmt.Errorf("error setting up DNS: %w", err)
		}
		return nil
	})

	// registry certs
	a.Add(l.copyCerts)

	// cross-platform emulation
	a.Add(func() error {
		// use binfmt when emulation is disabled i.e. host arch
		if conf.Binfmt != nil && *conf.Binfmt {
			if arch := environment.HostArch(); arch == environment.Arch(conf.Arch).Value() {
				if err := core.SetupBinfmt(l.host, l, environment.Arch(conf.Arch)); err != nil {
					logrus.Warn(fmt.Errorf("unable to enable qemu %s emulation: %w", arch, err))
				}
			}
		}

		if l.limaConf.VMOpts.VZOpts.Rosetta.Enabled {
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
		}

		return nil
	})

	// replicate addresses when network address is disabled
	a.Add(func() error {
		if err := l.replicateHostAddresses(conf); err != nil {
			logrus.Warnln(fmt.Errorf("unable to assign host IP addresses to the VM: %w", err))
		}
		return nil
	})

	// preserve state
	a.Add(func() error {
		if err := configmanager.SaveToFile(conf, config.CurrentProfile().StateFile()); err != nil {
			logrus.Warnln(fmt.Errorf("error persisting Colima state: %w", err))
		}
		return nil
	})

	// save store settings
	a.Add(func() error {
		if len(l.limaConf.AdditionalDisks) == 0 {
			return nil
		}

		// startup is successful
		// if additional disk is present, then it must've been formatted correctly.
		if err := store.Set(func(s *store.Store) {
			s.DiskFormatted = true
		}); err != nil {
			// not fatal, but should be logged
			logrus.Warnln(fmt.Errorf("error persisting store settings: %w", err))
		}

		return nil
	})

}

func (l *limaVM) assertQemu() error {
	// assert qemu requirement
	sameArchitecture := environment.HostArch() == l.limaConf.Arch
	if err := util.AssertQemuImg(); err != nil && l.limaConf.VMType == limaconfig.QEMU {
		if !sameArchitecture {
			return fmt.Errorf("qemu is required to emulate %s: %w", l.limaConf.Arch, err)
		}
		return err
	}
	return nil
}

const envLimaSSHPortForwarder = "LIMA_SSH_PORT_FORWARDER"

func (l *limaVM) prepareHost(conf config.Config) {
	useSSHPortForwarder := conf.PortForwarder != "grpc"

	l.host = l.host.WithEnv(envLimaSSHPortForwarder + "=" + strconv.FormatBool(useSSHPortForwarder))
}

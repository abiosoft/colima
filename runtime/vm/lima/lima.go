package lima

import (
	_ "embed"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/runtime"
	"github.com/abiosoft/colima/runtime/vm"
	"github.com/abiosoft/colima/util"
	"os"
	"path/filepath"
)

// New creates a new virtual machine VM.
func New(host runtime.HostActions, c config.Config) vm.VM {
	env := []string{limaInstanceEnvVar + "=" + config.AppName()}

	// consider making this truly flexible to support other VMs
	return &limaVM{
		conf:         c,
		host:         host.WithEnv(env),
		CommandChain: cli.New("vm"),
	}
}

const (
	limaInstanceEnvVar = "LIMA_INSTANCE"
	lima               = "lima"
	limactl            = "limactl"
)

func limaConfDir() string {
	home := util.HomeDir()
	return filepath.Join(home, ".lima", config.AppName())
}

func isConfigured() bool {
	stat, err := os.Stat(limaConfDir())
	return err == nil && stat.IsDir()
}

var _ vm.VM = (*limaVM)(nil)

type limaVM struct {
	conf config.Config
	host runtime.HostActions
	cli.CommandChain
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

func (l limaVM) Start() error {
	r := l.Init()

	if isConfigured() {
		return l.resume()
	}

	r.Stage("creating and starting")

	configFile := "colima.yaml"

	r.Add(func() error {
		limaConf := newConf(l.conf)
		return util.WriteYAML(limaConf, configFile)
	})
	r.Add(func() error {
		return l.host.Run(limactl, "start", "--tty=false", configFile)
	})
	r.Add(func() error {
		return os.Remove(configFile)
	})

	l.applyDNS(r)

	return r.Exec()
}

func (l limaVM) resume() error {
	r := l.Init()

	if l.Running() {
		r.Println("already running")
		return nil
	}

	configFile := filepath.Join(limaConfDir(), "lima.yaml")

	r.Add(func() error {
		limaConf := newConf(l.conf)
		return util.WriteYAML(limaConf, configFile)
	})

	r.Stage("starting")
	r.Add(func() error {
		return l.host.Run(limactl, "start", config.AppName())
	})

	l.applyDNS(r)

	return r.Exec()
}

func (l limaVM) applyDNS(r *cli.ActiveCommandChain) {
	// manually set the domain using systemd-resolve.
	//
	// Lima's DNS settings is fixed at VM create and cannot be changed afterwards.
	// this is a better approach as it only applies on VM startup and gets reset at shutdown.
	// this is specific to ubuntu, may be different for other distros.

	if len(l.conf.VM.DNS) == 0 {
		return
	}

	r.Stage("applying DNS config")

	// apply settings
	r.Add(func() error {
		args := []string{"sudo", "systemd-resolve", "--interface", "eth0"}
		for _, ip := range l.conf.VM.DNS {
			args = append(args, "--set-dns", ip.String())
		}
		return l.Run(args...)
	})
	// restart service, should not be needed but to ascertain
	r.Add(func() error {
		return l.Run("sudo", "systemctl", "restart", "systemd-resolved")
	})
}

func (l limaVM) Running() bool {
	return l.Run("uname") == nil
}

func (l limaVM) Stop() error {
	r := l.Init()
	if !l.Running() {
		r.Println("not running")
		return nil
	}

	r.Stage("stopping")

	r.Add(func() error {
		return l.host.Run(limactl, "stop", config.AppName())
	})

	return r.Exec()
}

func (l limaVM) Teardown() error {
	r := l.Init()
	if l.Running() {
		// lima needs to be stopped before it can be deleted.
		if err := l.Stop(); err != nil {
			return err
		}
	}

	r.Stage("deleting")

	r.Add(func() error {
		return l.host.Run(limactl, "delete", config.AppName())
	})

	return r.Exec()
}

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	r := l.Init()

	r.Add(func() error {
		return l.host.Run(args...)
	})

	return r.Exec()
}

func (l limaVM) RunInteractive(args ...string) error {
	args = append([]string{lima}, args...)

	r := l.Init()

	r.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return r.Exec()
}

func (l limaVM) RunOutput(args ...string) (out string, err error) {
	args = append([]string{lima}, args...)

	r := l.Init()

	r.Add(func() (err error) {
		out, err = l.host.RunOutput(args...)
		return
	})

	err = r.Exec()
	return
}

func (l limaVM) Host() runtime.HostActions {
	return l.host
}

package vm

import (
	_ "embed"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cli/runner"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/cutil"
	"os"
	"path/filepath"
)

type VM interface {
	cli.GuestActions
	Teardown() error
	Host() cli.HostActions
}

type Config struct {
	CPU     int
	Disk    int
	Memory  int
	SSHPort int

	// true when user changes config with flag
	Changed bool
}

// New creates a new VM.
func New(c Config) VM {
	env := []string{limaInstanceEnvVar + "=" + config.AppName()}

	// consider making this truly flexible to support other VMs
	return &limaVM{
		conf: c,
		r:    runner.New("vm"),
		host: host{env: env},
	}
}

const (
	limaInstanceEnvVar = "LIMA_INSTANCE"
	lima               = "lima"
	limactl            = "limactl"
)

func limaConfDir() string {
	home := cutil.HomeDir()
	return filepath.Join(home, ".lima", config.AppName())
}

func isConfigured() bool {
	stat, err := os.Stat(limaConfDir())
	return err == nil && stat.IsDir()
}

//go:embed vm.yaml
var limaConf string

var _ VM = (*limaVM)(nil)

type limaVM struct {
	conf Config
	r    *runner.Runner
	host cli.HostActions
}

func (l limaVM) Start() error {

	if isConfigured() {
		return l.resume()
	}

	l.r.Stage("creating and starting")

	configFile := "colima.yaml"

	var values = struct {
		Config
		User string
	}{Config: l.conf, User: cutil.User()}

	l.r.Add(func() error {
		return cutil.WriteTemplate(limaConf, configFile, values)
	})
	l.r.Add(func() error {
		return l.host.Run(limactl, "start", "--tty=false", configFile)
	})
	l.r.Add(func() error {
		return os.Remove(configFile)
	})

	return l.r.Run()
}

func (l limaVM) resume() error {
	if l.isRunning() {
		l.r.Println("already running")
		return nil
	}

	if l.conf.Changed {
		l.r.Stage("config change detected, updating")
		configFile := filepath.Join(limaConfDir(), "lima.yaml")

		var values = struct {
			Config
			User string
		}{Config: l.conf, User: cutil.User()}

		l.r.Add(func() error {
			return cutil.WriteTemplate(limaConf, configFile, values)
		})
	}

	l.r.Stage("starting")
	l.r.Add(func() error {
		return l.host.Run(limactl, "start")
	})
	return l.r.Run()
}

func (l limaVM) isRunning() bool {
	return l.host.Run(lima, "uname") == nil
}

func (l limaVM) Stop() error {
	l.r.Stage("stopping")

	l.r.Add(func() error {
		return l.host.Run(limactl, "stop")
	})

	return l.r.Run()
}

func (l limaVM) Teardown() error {
	l.r.Stage("deleting")

	l.r.Add(func() error {
		return l.host.Run(limactl, "delete", config.AppName())
	})

	return l.r.Run()
}

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	l.r.Add(func() error {
		return l.host.Run(args...)
	})

	return l.r.Run()
}

func (l limaVM) Host() cli.HostActions {
	return l.host
}

var _ cli.HostActions = (*host)(nil)

type host struct {
	env []string
}

func (h host) Run(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	cmd := cli.NewCommand(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	return cmd.Run()
}

package vm

import (
	_ "embed"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/runner"
	"github.com/abiosoft/colima/runtime"
	"github.com/abiosoft/colima/runtime/host"
	"github.com/abiosoft/colima/util"
	"os"
	"path/filepath"
)

// Runtime is virtual machine runtime.
type Runtime interface {
	runtime.GuestActions
	runtime.Dependencies
	Teardown() error
}

type Config struct {
	CPU     int
	Disk    int
	Memory  int
	SSHPort int

	// true when user changes config with flag
	Changed bool
}

// New creates a new virtual machine Runtime.
func New(c Config) Runtime {
	env := []string{limaInstanceEnvVar + "=" + config.AppName()}

	// consider making this truly flexible to support other VMs
	return &limaVM{
		conf:     c,
		host:     host.New(env),
		Instance: runner.New("vm"),
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

//go:embed vm.yaml
var limaConf string

var _ Runtime = (*limaVM)(nil)

type limaVM struct {
	conf Config
	host runtime.HostActions
	runner.Instance
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

	var values = struct {
		Config
		User string
	}{Config: l.conf, User: util.User()}

	r.Add(func() error {
		return util.WriteTemplate(limaConf, configFile, values)
	})
	r.Add(func() error {
		return l.host.Run(limactl, "start", "--tty=false", configFile)
	})
	r.Add(func() error {
		return os.Remove(configFile)
	})

	return r.Run()
}

func (l limaVM) resume() error {
	r := l.Init()
	if l.isRunning() {
		r.Println("already running")
		return nil
	}

	if l.conf.Changed {
		r.Stage("config change detected, updating")
		configFile := filepath.Join(limaConfDir(), "lima.yaml")

		var values = struct {
			Config
			User string
		}{Config: l.conf, User: util.User()}

		r.Add(func() error {
			return util.WriteTemplate(limaConf, configFile, values)
		})
	}

	r.Stage("starting")
	r.Add(func() error {
		return l.host.Run(limactl, "start")
	})
	return r.Run()
}

func (l limaVM) isRunning() bool {
	return l.host.Run(lima, "uname") == nil
}

func (l limaVM) Stop() error {
	r := l.Init()

	r.Stage("stopping")

	r.Add(func() error {
		return l.host.Run(limactl, "stop")
	})

	return r.Run()
}

func (l limaVM) Teardown() error {
	r := l.Init()

	r.Stage("deleting")

	r.Add(func() error {
		return l.host.Run(limactl, "delete", config.AppName())
	})

	return r.Run()
}

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	r := l.Init()

	r.Add(func() error {
		return l.host.Run(args...)
	})

	return r.Run()
}

func (l limaVM) Host() runtime.HostActions {
	return l.host
}

package lima

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/yamlutil"
)

// New creates a new virtual machine.
func New(host environment.HostActions) environment.VM {
	env := limaInstanceEnvVar + "=" + config.Profile().ID

	// consider making this truly flexible to support other VMs
	return &limaVM{
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
	return filepath.Join(home, ".lima", config.Profile().ID)
}

var _ environment.VM = (*limaVM)(nil)

type limaVM struct {
	host environment.HostActions
	cli.CommandChain

	// keep config in case of restart
	conf config.Config
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

func (l *limaVM) Start(conf config.Config) error {
	a := l.Init()

	if l.Created() {
		return l.resume(conf)
	}

	a.Stage("creating and starting")

	configFile := config.Profile().ID + ".yaml"

	a.Add(func() error {
		limaConf, err := newConf(conf)
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

	l.applyDNS(a, conf)

	// adding it to command chain to execute only after successful startup.
	a.Add(func() error {
		l.conf = conf
		return nil
	})

	return a.Exec()
}

func (l limaVM) resume(conf config.Config) error {
	log := l.Logger()
	a := l.Init()

	if l.Running() {
		log.Println("already running")
		return nil
	}

	configFile := filepath.Join(limaConfDir(), "lima.yaml")

	a.Add(func() error {
		limaConf, err := newConf(conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(limaConf, configFile)
	})

	a.Stage("starting")
	a.Add(func() error {
		return l.host.Run(limactl, "start", config.Profile().ID)
	})

	l.applyDNS(a, conf)

	return a.Exec()
}

func (l limaVM) applyDNS(a *cli.ActiveCommandChain, conf config.Config) {
	// manually set the domain using systemd-resolve.
	//
	// Lima's DNS settings is fixed at VM create and cannot be changed afterwards.
	// this is a better approach as it only applies on VM startup and gets reset at shutdown.
	// this is specific to ubuntu, may be different for other distros.

	if len(conf.VM.DNS) == 0 {
		return
	}

	a.Stage("applying DNS config")

	// apply settings
	a.Add(func() error {
		args := []string{"sudo", "systemd-resolve", "--interface", "eth0"}
		for _, ip := range conf.VM.DNS {
			args = append(args, "--set-dns", ip.String())
		}
		return l.Run(args...)
	})
	// restart service, should not be needed but to ascertain
	a.Add(func() error {
		return l.Run("sudo", "systemctl", "restart", "systemd-resolved")
	})
}

func (l limaVM) Running() bool {
	return l.RunQuiet("uname") == nil
}

func (l limaVM) Stop() error {
	log := l.Logger()
	a := l.Init()
	if !l.Running() {
		log.Println("not running")
		return nil
	}

	a.Stage("stopping")

	a.Add(func() error {
		return l.host.Run(limactl, "stop", config.Profile().ID)
	})

	return a.Exec()
}

func (l limaVM) Teardown() error {
	a := l.Init()
	if l.Running() {
		// lima needs to be stopped before it can be deleted.
		if err := l.Stop(); err != nil {
			return err
		}
	}

	a.Stage("deleting")

	a.Add(func() error {
		return l.host.Run(limactl, "delete", config.Profile().ID)
	})

	return a.Exec()
}

func (l limaVM) Restart() error {
	if l.conf.Empty() {
		return fmt.Errorf("cannot restart, VM not previously started")
	}

	if err := l.Stop(); err != nil {
		return err
	}

	// minor delay to prevent possible race condition.
	time.Sleep(time.Second * 2)

	if err := l.Start(l.conf); err != nil {
		return err
	}

	return nil
}

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init()

	a.Add(func() error {
		return l.host.Run(args...)
	})

	return a.Exec()
}

func (l limaVM) RunInteractive(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init()

	a.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return a.Exec()
}

func (l limaVM) RunBackground(args ...string) error {
	args = append([]string{lima}, args...)
	a := l.Init()

	a.Add(func() error {
		return l.host.RunBackground(args...)
	})

	return a.Exec()
}

func (l limaVM) RunOutput(args ...string) (out string, err error) {
	args = append([]string{lima}, args...)

	a := l.Init()

	a.Add(func() (err error) {
		out, err = l.host.RunOutput(args...)
		return
	})

	err = a.Exec()
	return
}

func (l limaVM) RunQuiet(args ...string) (err error) {
	args = append([]string{lima}, args...)

	a := l.Init()

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
	if !l.Running() {
		return "", fmt.Errorf("not running")
	}
	return l.RunOutput("echo", "$"+s)
}

func (l limaVM) Created() bool {
	stat, err := os.Stat(limaConfDir())
	return err == nil && stat.IsDir()
}

const configFile = "/etc/colima/colima.json"

func (l limaVM) getConf() map[string]string {
	obj := map[string]string{}
	b, err := l.RunOutput("cat", configFile)
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
	if err := l.Run("sudo", "sh", "-c", fmt.Sprintf(`echo %s > %s`, strconv.Quote(string(b)), configFile)); err != nil {
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

package lima

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/network"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/yamlutil"
	"github.com/sirupsen/logrus"
)

// New creates a new virtual machine.
func New(host environment.HostActions) environment.VM {
	env := limaInstanceEnvVar + "=" + config.Profile().ID

	home, err := limaHome()
	if err != nil {
		err = fmt.Errorf("error detecting Lima config directory: %w", err)
		logrus.Warnln(err)
		logrus.Warnln("falling back to default '$HOME/.lima'")
		home = filepath.Join(util.HomeDir(), ".lima")
	}

	// consider making this truly flexible to support other VMs
	return &limaVM{
		host:         host.WithEnv(env),
		home:         home,
		CommandChain: cli.New("vm"),
		network:      network.NewManager(host),
	}
}

const (
	limaInstanceEnvVar = "LIMA_INSTANCE"
	lima               = "lima"
	limactl            = "limactl"
)

func limaHome() (string, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "info")
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error retrieving lima info: %w", err)
	}

	var resp struct {
		LimaHome string `json:"limaHome"`
	}
	if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
		return "", fmt.Errorf("error decoding json for lima info: %w", err)
	}
	if resp.LimaHome == "" {
		return "", fmt.Errorf("error retrieving lima info, ensure lima version is >0.7.4")
	}

	return resp.LimaHome, nil
}

func (l limaVM) limaConfDir() string {
	return filepath.Join(l.home, config.Profile().ID)
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
	network network.NetworkManager
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

var ctxKeyNetwork = struct{ name string }{name: "network"}

func (l limaVM) prepareNetwork(ctx cli.Context) error {
	// limited to macOS for now
	if runtime.GOOS != "darwin" {
		return nil
	}

	// use a nested chain for convenience
	a := l.Init()
	log := l.Logger()

	a.Stage("preparing network")
	a.Add(func() error {
		if !l.network.DependenciesInstalled() {
			log.Println("network dependencies missing")
			log.Println("sudo password may be required for setting up network dependencies")
			return l.network.InstallDependencies()
		}
		return nil
	})
	a.Add(l.network.Start)

	// delay to ensure that the network is running
	a.Retry("", time.Second*1, 15, func() error {
		ok, err := l.network.Running()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("network process is not running")
		}
		return err
	})

	// network failure is not fatal
	if err := a.Exec(); err != nil {
		log.Warnln(fmt.Errorf("error starting network: %w", err))
	} else {
		ctx.SetContext(context.WithValue(ctx, ctxKeyNetwork, true))
	}

	return nil
}

func (l *limaVM) Start(conf config.Config) error {
	a := l.Init()

	if l.Created() {
		return l.resume(conf)
	}

	a.AddCtx(l.prepareNetwork)
	// l.prepareNetwork()

	a.Stage("creating and starting")
	configFile := filepath.Join(os.TempDir(), config.Profile().ID+".yaml")

	a.AddCtx(func(ctx cli.Context) error {
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

	// registry certs
	a.Add(l.copyCerts)

	// dns
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

	a.AddCtx(l.prepareNetwork)

	configFile := filepath.Join(l.limaConfDir(), "lima.yaml")

	a.AddCtx(func(ctx cli.Context) error {
		limaConf, err := newConf(ctx, conf)
		if err != nil {
			return err
		}
		return yamlutil.WriteYAML(limaConf, configFile)
	})

	a.Stage("starting")
	a.Add(func() error {
		return l.host.Run(limactl, "start", config.Profile().ID)
	})

	// registry certs
	a.Add(l.copyCerts)

	l.applyDNS(a, conf)

	return a.Exec()
}

func (l limaVM) applyDNS(a *cli.ActiveCommandChain, conf config.Config) {
	// manually set the DNS by modifying the resolve file.
	//
	// Lima's DNS settings is fixed at VM create and cannot be changed afterwards.
	// this is a better approach as it only applies on VM startup and gets reset at shutdown.
	// this is specific to Alpine , may be different for other distros.
	log := l.Logger()
	dnsFile := "/etc/resolv.conf"
	dnsFileBak := dnsFile + ".lima"

	a.Add(func() error {
		// backup the original dns file (if not previously done)
		if l.RunQuiet("stat", dnsFileBak) != nil {
			err := l.RunQuiet("sudo", "cp", dnsFile, dnsFileBak)
			if err != nil {
				// custom DNS config failure should not prevent the VM from starting
				// as the default config will be used.
				// Rather, warn and terminate setting the DNS config.
				log.Warnln(fmt.Errorf("error backing up default DNS config: %w", err))
				return nil
			}
		}
		return nil
	})

	a.Add(func() error {
		// empty the file
		if err := l.RunQuiet("sudo", "rm", "-f", dnsFile); err != nil {
			return fmt.Errorf("error initiating DNS config: %w", err)
		}

		for _, dns := range conf.VM.DNS {
			line := fmt.Sprintf(`echo nameserver %s >> %s`, dns.String(), dnsFile)
			if err := l.RunQuiet("sudo", "sh", "-c", line); err != nil {
				return fmt.Errorf("error applying DNS config: %w", err)
			}
		}

		if len(conf.VM.DNS) > 0 {
			return nil
		}

		// use the default Lima dns if no dns is set
		return l.RunQuiet("sudo", "sh", "-c", fmt.Sprintf("cat %s >> %s", dnsFileBak, dnsFile))
	})
}

func (l limaVM) Running() bool {
	return l.RunQuiet("uname") == nil
}

func (l limaVM) Stop(force bool) error {
	log := l.Logger()
	a := l.Init()
	if !l.Running() {
		log.Println("not running")
		return nil
	}

	a.Stage("stopping")

	a.Add(func() error {
		if force {
			return l.host.Run(limactl, "stop", "--force", config.Profile().ID)
		}
		return l.host.Run(limactl, "stop", config.Profile().ID)
	})

	a.Add(l.network.Stop)

	return a.Exec()
}

func (l limaVM) Teardown() error {
	a := l.Init()

	a.Stage("deleting")

	a.Add(func() error {
		return l.host.Run(limactl, "delete", "--force", config.Profile().ID)
	})

	a.Add(l.network.Stop)

	return a.Exec()
}

func (l limaVM) Restart() error {
	if l.conf.Empty() {
		return fmt.Errorf("cannot restart, VM not previously started")
	}

	if err := l.Stop(false); err != nil {
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
	stat, err := os.Stat(l.limaConfDir())
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

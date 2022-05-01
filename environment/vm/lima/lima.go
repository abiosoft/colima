package lima

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/gvproxy"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/vmnet"

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
	var envs []string
	envLimaInstance := limaInstanceEnvVar + "=" + config.Profile().ID
	envSubprocess := config.SubprocessProfileEnvVar + "=" + config.Profile().ShortName
	envs = append(envs, envLimaInstance, envSubprocess)

	binDir := filepath.Join(config.WrapperDir(), "bin")
	envs = append(envs, "PATH="+util.AppendToPath(os.Getenv("PATH"), binDir))

	home, err := limaHome()
	if err != nil {
		err = fmt.Errorf("error detecting Lima config directory: %w", err)
		logrus.Warnln(err)
		logrus.Warnln("falling back to default '$HOME/.lima'")
		home = filepath.Join(util.HomeDir(), ".lima")
	}

	// consider making this truly flexible to support other VMs
	return &limaVM{
		host:         host.WithEnv(envs...),
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
	network network.Manager
}

func (l limaVM) Dependencies() []string {
	return []string{
		"lima",
	}
}

func (l *limaVM) prepareNetwork(ctx context.Context, conf config.Network) (context.Context, error) {
	// limited to macOS for now
	if !util.MacOS() {
		return ctx, nil
	}

	ctxKeyVmnet := network.CtxKey(vmnet.Name())
	ctxKeyGVProxy := network.CtxKey(gvproxy.Name())

	// use a nested chain for convenience
	a := l.Init()
	log := l.Logger()

	a.Stage("preparing network")
	a.Add(func() error {
		if conf.Address {
			ctx = context.WithValue(ctx, ctxKeyVmnet, true)
		}
		if conf.Gateway == config.GVProxyGateway {
			ctx = context.WithValue(ctx, ctxKeyGVProxy, true)
		}
		deps, root := l.network.Dependencies(ctx)
		if deps.Installed() {
			return nil
		}

		// if user interaction is not required (i.e. root),
		// no need for another verbose info.
		if root {
			log.Println("network dependencies missing")
			log.Println("sudo password may be required for setting up network dependencies")
		}
		return deps.Install(l.host)
	})

	a.Add(func() error {
		return l.network.Start(ctx)
	})

	// delay to ensure that the vmnet is running
	statusKey := "networkStatus"
	if conf.Address || conf.Gateway == config.VmnetGateway {
		a.Retry("", time.Second*3, 5, func(i int) error {
			s, err := l.network.Running(ctx)
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
			status, ok := ctx.Value(statusKey).(network.Status)
			if !ok {
				return
			}
			if !status.Running {
				log.Warnln(fmt.Errorf("error starting network: %w", err))
				return
			}

			for _, p := range status.Processes {
				if !p.Running {
					ctx = context.WithValue(ctx, network.CtxKey(p.Name), false)
					log.Warnln(fmt.Errorf("error starting %s: %w", p.Name, err))
				}
			}
		}()
	}

	// preserve gvproxy context
	if gvproxyEnabled, _ := ctx.Value(network.CtxKey(gvproxy.Name())).(bool); gvproxyEnabled {
		l.host = l.host.WithEnv(gvproxy.SubProcessEnvVar + "=1")
	}

	return ctx, nil
}

func (l *limaVM) Start(ctx context.Context, conf config.Config) error {
	a := l.Init()

	if l.Created() {
		return l.resume(ctx, conf)
	}

	a.Add(func() (err error) {
		ctx, err = l.prepareNetwork(ctx, conf.Network)
		return err
	})

	a.Stage("creating and starting")
	configFile := filepath.Join(os.TempDir(), config.Profile().ID+".yaml")

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

	// registry certs
	a.Add(l.copyCerts)

	// dns
	l.applyDNS(ctx, a, conf)

	// adding it to command chain to execute only after successful startup.
	a.Add(func() error {
		l.conf = conf
		return nil
	})

	return a.Exec()
}

func (l limaVM) resume(ctx context.Context, conf config.Config) error {
	log := l.Logger()
	a := l.Init()

	if l.Running() {
		log.Println("already running")
		return nil
	}

	a.Add(func() (err error) {
		ctx, err = l.prepareNetwork(ctx, conf.Network)
		return err
	})

	configFile := filepath.Join(l.limaConfDir(), "lima.yaml")

	a.Add(func() error {
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

	l.applyDNS(ctx, a, conf)

	return a.Exec()
}

func (l *limaVM) applyDNS(ctx context.Context, a *cli.ActiveCommandChain, conf config.Config) {
	// Lima's DNS settings is fixed at VM create and cannot be changed afterwards.
	// this is a better approach as it only applies on VM startup and gets reset at shutdown.
	log := l.Logger()

	dns := network.NewDNSManager(l)
	a.Add(func() error {
		var dnses []net.IP
		dnses = append(dnses, conf.DNS...)

		// check if network is enabled
		if enabled, _ := ctx.Value(network.CtxKey(vmnet.Name())).(bool); enabled && len(dnses) == 0 {
			dnses = append(dnses, net.ParseIP(vmnet.NetGateway))
		}
		switch conf.Network.Gateway {
		case config.VmnetGateway:
			dnses = append(dnses, net.ParseIP(vmnet.NetGateway))
		case config.GVProxyGateway:
			dnses = append(dnses, net.ParseIP(gvproxy.GatewayIP))
		}

		// custom DNS config failure should not prevent the VM from starting
		// as the default config will be used.
		// Rather, warn and terminate setting the DNS config.
		if err := dns.Provision(dnses); err != nil {
			log.Warnln(fmt.Errorf("error provisioning dns, will fall back to defaults: %w", err))
		}
		if err := dns.Start(); err != nil {
			log.Warnln(fmt.Errorf("error starting dns, will fall back to defaults: %w", err))
		}
		return nil
	})
}

func (l limaVM) Running() bool {
	return l.RunQuiet("uname") == nil
}

func (l limaVM) Stop(ctx context.Context, force bool) error {
	log := l.Logger()
	a := l.Init()
	if !l.Running() {
		log.Println("not running")
		return nil
	}

	a.Stage("stopping")

	if util.MacOS() {
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			return l.network.Stop(ctx)
		})
	}

	a.Add(func() error {
		if force {
			return l.host.Run(limactl, "stop", "--force", config.Profile().ID)
		}
		return l.host.Run(limactl, "stop", config.Profile().ID)
	})

	return a.Exec()
}

func (l limaVM) Teardown(ctx context.Context) error {
	a := l.Init()

	a.Stage("deleting")

	if util.MacOS() {
		a.Retry("", time.Second*1, 10, func(retryCount int) error {
			return l.network.Stop(ctx)
		})
	}

	a.Add(func() error {
		return l.host.Run(limactl, "delete", "--force", config.Profile().ID)
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

func (l limaVM) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init()

	a.Add(func() error {
		return l.host.RunWith(stdin, stdout, args...)
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

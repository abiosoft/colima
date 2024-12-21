package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
)

// Name is container runtime name

const (
	Name              = "kubernetes"
	DefaultK3sVersion = "v1.31.2+k3s1"
	DefaultK0sVersion = "v1.31.3+k0s.0"
	ConfigKey         = "kubernetes_config"
)

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &kubernetesRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func init() {
	environment.RegisterContainer(Name, newRuntime, true)
}

var _ environment.Container = (*kubernetesRuntime)(nil)

type kubernetesRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

func (c kubernetesRuntime) Name() string {
	return Name
}

func (c kubernetesRuntime) isInstalled(useK0s bool) bool {
	if useK0s {
		return c.guest.RunQuiet("command", "-v", "k0s") == nil
	}
	// it is installed if uninstall script is present.
	return c.guest.RunQuiet("command", "-v", "k3s-uninstall.sh") == nil
}

func (c kubernetesRuntime) isVersionInstalled(version string, useK0s bool) bool {
	if useK0s {
		out, err := c.guest.RunOutput("k0s", "version")
		if err != nil {
			return false
		}
		return strings.Contains(out, version)
	} else {
		// validate version change via cli flag/config.
		out, err := c.guest.RunOutput("k3s", "--version")
		if err != nil {
			return false
		}
		return strings.Contains(out, version)
	}
}

func (c kubernetesRuntime) Running(context.Context) bool {
	if c.config().UseK0s {
		return c.guest.RunQuiet("sudo", "service", "k0scontroller", "status") == nil
	}
	return c.guest.RunQuiet("sudo", "service", "k3s", "status") == nil
}

func (c kubernetesRuntime) runtime() string {
	return c.guest.Get(environment.ContainerRuntimeKey)
}

func (c kubernetesRuntime) config() config.Kubernetes {
	conf := config.Kubernetes{}
	if b := c.guest.Get(ConfigKey); b != "" {
		_ = json.Unmarshal([]byte(b), &conf)
	}

	// Set default version based on UseK0s flag
	if conf.Version == "" {
		if conf.UseK0s {
			conf.Version = DefaultK0sVersion
		} else {
			conf.Version = DefaultK3sVersion
		}
	}

	return conf
}

func (c kubernetesRuntime) setConfig(conf config.Kubernetes) error {
	b, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error encoding kubernetes config to json: %w", err)
	}

	return c.guest.Set(ConfigKey, string(b))
}

func (c *kubernetesRuntime) Provision(ctx context.Context) error {
	log := c.Logger(ctx)
	a := c.Init(ctx)
	if c.Running(ctx) {
		return nil
	}

	appConf, ok := ctx.Value(config.CtxKey()).(config.Config)
	runtime := appConf.Runtime
	conf := appConf.Kubernetes

	if !ok {
		// this should be a restart/start while vm is active
		// retrieve value in the vm
		runtime = c.runtime()
		conf = c.config()
	}

	if c.isVersionInstalled(conf.Version, conf.UseK0s) {
		// runtime has changed, ensure the required images are in the registry
		if currentRuntime := c.runtime(); currentRuntime != "" && currentRuntime != runtime {
			if !conf.UseK0s {
				a.Stagef("changing runtime to %s", runtime)
				// other settings may have changed e.g. ingress
				installK3sCache(c.host, c.guest, a, log, runtime, conf.Version)
			}
		}
	} else {
		if c.isInstalled(conf.UseK0s) {
			a.Stagef("version changed to %s, downloading and installing", conf.Version)
		} else {
			if ok {
				a.Stage("downloading and installing")
			} else {
				a.Stage("installing")
			}
		}
		if conf.UseK0s {
			installK0s(c.guest, a, conf.Version)
		} else {
			installK3s(c.host, c.guest, a, log, runtime, conf.Version, conf.K3sArgs)
		}
	}

	// this needs to happen on each startup
	{
		if !conf.UseK0s {
			// cni is used by both cri-dockerd and containerd
			installCniConfig(c.guest, a)
		}
	}

	// provision successful, now we can persist the version
	a.Add(func() error { return c.setConfig(conf) })

	return a.Exec()
}

func (c kubernetesRuntime) Start(ctx context.Context) error {
	log := c.Logger(ctx)
	a := c.Init(ctx)
	if c.Running(ctx) {
		log.Println("already running")
		return nil
	}

	if c.config().UseK0s {
		a.Add(func() error {
			return c.guest.Run("sudo", "systemctl", "start", "k0scontroller")
		})
		a.Retry("", time.Second*2, 10, func(int) error {
			return c.guest.RunQuiet("sudo", "k0s", "kubectl", "cluster-info")
		})
	} else {
		a.Add(func() error {
			return c.guest.Run("sudo", "service", "k3s", "start")
		})
		a.Retry("", time.Second*2, 10, func(int) error {
			return c.guest.RunQuiet("kubectl", "cluster-info")
		})
	}

	if err := a.Exec(); err != nil {
		return err
	}

	return c.provisionKubeconfig(ctx)
}

func (c kubernetesRuntime) Stop(ctx context.Context) error {
	a := c.Init(ctx)
	a.Add(func() error {
		return c.guest.Run("k3s-killall.sh")
	})

	// k3s is buggy with external containerd for now
	// cleanup is manual
	a.Add(c.stopAllContainers)

	return a.Exec()
}

func (c kubernetesRuntime) deleteAllContainers() error {
	ids := c.runningContainerIDs()
	if ids == "" {
		return nil
	}

	var args []string

	switch c.runtime() {
	case containerd.Name:
		args = []string{"nerdctl", "-n", "k8s.io", "rm", "-f"}
	case docker.Name:
		args = []string{"docker", "rm", "-f"}
	default:
		return nil
	}

	args = append(args, strings.Fields(ids)...)

	return c.guest.Run("sudo", "sh", "-c", strings.Join(args, " "))
}

func (c kubernetesRuntime) stopAllContainers() error {
	ids := c.runningContainerIDs()
	if ids == "" {
		return nil
	}

	var args []string

	switch c.runtime() {
	case containerd.Name:
		args = []string{"nerdctl", "-n", "k8s.io", "kill"}
	case docker.Name:
		args = []string{"docker", "kill"}
	default:
		return nil
	}

	args = append(args, strings.Fields(ids)...)

	return c.guest.Run("sudo", "sh", "-c", strings.Join(args, " "))
}

func (c kubernetesRuntime) runningContainerIDs() string {
	var args []string

	switch c.runtime() {
	case containerd.Name:
		args = []string{"sudo", "nerdctl", "-n", "k8s.io", "ps", "-q"}
	case docker.Name:
		args = []string{"sudo", "sh", "-c", `docker ps --format '{{.Names}}'| grep "k8s_"`}
	default:
		return ""
	}

	ids, _ := c.guest.RunOutput(args...)
	if ids == "" {
		return ""
	}
	return strings.ReplaceAll(ids, "\n", " ")
}

func (c kubernetesRuntime) Teardown(ctx context.Context) error {
	a := c.Init(ctx)

	if c.isInstalled(c.config().UseK0s) {
		a.Add(func() error {
			if c.config().UseK0s {
				if err := c.guest.Run("sudo", "systemctl", "stop", "k0scontroller"); err != nil {
					return fmt.Errorf("error stopping k0scontroller services: %w", err)
				}
				return c.guest.Run("sudo", "k0s", "reset")
			} else {
				return c.guest.Run("k3s-uninstall.sh")
			}
		})
	}

	if !c.config().UseK0s {
		// k3s is buggy with external containerd for now
		// cleanup is manual
		a.Add(c.deleteAllContainers)
	}

	c.teardownKubeconfig(a)

	return a.Exec()
}

func (c kubernetesRuntime) Dependencies() []string {
	return []string{"kubectl"}
}

func (c kubernetesRuntime) Version(context.Context) string {
	version, _ := c.host.RunOutput("kubectl", "--context", config.CurrentProfile().ID, "version", "--short")
	return version
}

func (c *kubernetesRuntime) Update(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("update not supported for the %s runtime", Name)
}

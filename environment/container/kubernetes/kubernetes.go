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
	Name           = "kubernetes"
	DefaultVersion = "v1.32.0+k3s1"

	ConfigKey = "kubernetes_config"
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

func (c kubernetesRuntime) isInstalled() bool {
	// it is installed if uninstall script is present.
	return c.guest.RunQuiet("command", "-v", "k3s-uninstall.sh") == nil
}

func (c kubernetesRuntime) isVersionInstalled(version string) bool {
	// validate version change via cli flag/config.
	out, err := c.guest.RunOutput("k3s", "--version")
	if err != nil {
		return false
	}
	return strings.Contains(out, version)
}

func (c kubernetesRuntime) Running(context.Context) bool {
	return c.guest.RunQuiet("sudo", "service", "k3s", "status") == nil
}

func (c kubernetesRuntime) runtime() string {
	return c.guest.Get(environment.ContainerRuntimeKey)
}

func (c kubernetesRuntime) config() config.Kubernetes {
	conf := config.Kubernetes{Version: DefaultVersion}
	if b := c.guest.Get(ConfigKey); b != "" {
		_ = json.Unmarshal([]byte(b), &conf)
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

	if conf.Version == "" {
		// this ensure if `version` tag in `kubernetes` section in yaml is empty,
		// it should assign with the `DefaultVersion` for the baseURL
		conf.Version = DefaultVersion
	}

	if c.isVersionInstalled(conf.Version) {
		// runtime has changed, ensure the required images are in the registry
		if currentRuntime := c.runtime(); currentRuntime != "" && currentRuntime != runtime {
			a.Stagef("changing runtime to %s", runtime)
			installK3sCache(c.host, c.guest, a, log, runtime, conf.Version)
		}
		// other settings may have changed e.g. ingress
		installK3sCluster(c.host, c.guest, a, runtime, conf.Version, conf.K3sArgs)
	} else {
		if c.isInstalled() {
			a.Stagef("version changed to %s, downloading and installing", conf.Version)
		} else {
			if ok {
				a.Stage("downloading and installing")
			} else {
				a.Stage("installing")
			}
		}
		installK3s(c.host, c.guest, a, log, runtime, conf.Version, conf.K3sArgs)
	}

	// this needs to happen on each startup
	{
		// cni is used by both cri-dockerd and containerd
		installCniConfig(c.guest, a)
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

	a.Add(func() error {
		return c.guest.Run("sudo", "service", "k3s", "start")
	})
	a.Retry("", time.Second*2, 10, func(int) error {
		return c.guest.RunQuiet("kubectl", "cluster-info")
	})

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

	if c.isInstalled() {
		a.Add(func() error {
			return c.guest.Run("k3s-uninstall.sh")
		})
	}

	// k3s is buggy with external containerd for now
	// cleanup is manual
	a.Add(c.deleteAllContainers)

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

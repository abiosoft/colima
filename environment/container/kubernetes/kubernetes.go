package kubernetes

import (
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
)

// Name is container runtime name
const Name = "kubernetes"

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &kubernetesRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func init() {
	environment.RegisterContainer(Name, newRuntime)
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
	if err := c.guest.RunQuiet("command", "-v", "k3s-uninstall.sh"); err != nil {
		return false
	}

	// validate version change via cli flag/config.
	// if version is different, it is as if it is not yet installed
	out, err := c.guest.RunOutput("k3s", "--version")
	if err != nil {
		return false
	}
	return strings.Contains(out, c.kubernetesVersion())
}

func (c kubernetesRuntime) Running() bool {
	return c.guest.RunQuiet("sudo", "service", "k3s", "status") == nil
}

func (c kubernetesRuntime) runtime() string {
	return c.guest.Get(environment.ContainerRuntimeKey)
}
func (c kubernetesRuntime) kubernetesVersion() string {
	return c.guest.Get(environment.KubernetesVersionKey)
}
func (c kubernetesRuntime) kubernetesIngressEnabled() bool {
	enabled, _ := strconv.ParseBool(c.guest.Get(environment.KubernetesIngressKey))
	return enabled
}

func (c *kubernetesRuntime) Provision() error {
	log := c.Logger()
	a := c.Init()

	if c.isInstalled() {
		// ingress settings may have changed
		installK3sCluster(c.host, c.guest, a, c.runtime(), c.kubernetesVersion(), c.kubernetesIngressEnabled())
	} else {
		a.Stage("downloading and installing")
		installK3s(c.host, c.guest, a, log, c.runtime(), c.kubernetesVersion(), c.kubernetesIngressEnabled())
	}

	// this needs to happen on each startup
	switch c.runtime() {
	case containerd.Name:
		installContainerdDeps(c.guest, a)
	case docker.Name:
		a.Retry("waiting for docker cri", time.Second*2, 5, func() error {
			return c.guest.Run("sudo", "service", "cri-dockerd", "start")
		})
	}

	return a.Exec()
}

func (c kubernetesRuntime) Start() error {
	log := c.Logger()
	a := c.Init()
	if c.Running() {
		log.Println("already running")
		return nil
	}

	a.Stage("starting")

	a.Add(func() error {
		return c.guest.Run("sudo", "service", "k3s", "start")
	})
	a.Retry("", time.Second*2, 10, func() error {
		return c.guest.RunQuiet("kubectl", "cluster-info")
	})

	if err := a.Exec(); err != nil {
		return err
	}

	return c.provisionKubeconfig()
}

func (c kubernetesRuntime) Stop() error {
	a := c.Init()
	a.Stage("stopping")
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

func (c kubernetesRuntime) Teardown() error {
	a := c.Init()
	a.Stage("deleting")

	if c.isInstalled() {
		a.Add(func() error {
			return c.guest.Run("k3s-uninstall.sh")
		})
	}

	// k3s is buggy with external containerd for now
	// cleanup is manual
	a.Add(func() error {
		return c.deleteAllContainers()
	})

	c.teardownKubeconfig(a)
	a.Add(func() error {
		return c.guest.Set(kubeconfigKey, "")
	})

	return a.Exec()
}

func (c kubernetesRuntime) Dependencies() []string {
	return []string{"kubectl"}
}

func (c kubernetesRuntime) Version() string {
	version, _ := c.host.RunOutput("kubectl", "--context", config.Profile().ID, "version", "--short")
	return version
}

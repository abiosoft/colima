package kubernetes

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
)

// Name is container runtime name
const Name = "kubernetes"

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &kubernetesRuntime{
		host:         host.WithEnv("MINIKUBE_IN_STYLE=0"),
		guest:        guest,
		CommandChain: cli.New("kubernetes"),
	}
}

func init() {
	environment.RegisterContainer(Name, newRuntime)
}

var _ environment.Container = (*kubernetesRuntime)(nil)

type kubernetesRuntime struct {
	host             environment.HostActions
	guest            environment.GuestActions
	newlyProvisioned bool // track first run
	cli.CommandChain
}

func (c kubernetesRuntime) Name() string {
	return Name
}

func (c kubernetesRuntime) isInstalled() bool {
	// minikube is the last provision step.
	// if it is present, everything is assumed fine.
	return c.guest.Run("command", "-v", "minikube") == nil
}

func (c kubernetesRuntime) Running() bool {
	// minikube is the last provision step.
	// if it is present, everything is assumed fine.
	return c.guest.Run("minikube", "status") == nil
}

func (c kubernetesRuntime) runtime() string {
	return c.guest.Get(environment.ContainerRuntimeKey)
}
func (c kubernetesRuntime) kubernetesVersion() string {
	return c.guest.Get(environment.KubernetesVersionKey)
}

func (c *kubernetesRuntime) Provision() error {
	r := c.Init()

	if c.isInstalled() {
		return nil
	}

	r.Stage("provisioning")

	// apt install deps
	r.Stage("installing dependencies")
	r.Add(func() error {
		return c.guest.Run("sudo", "apt", "install", "-y", "conntrack", "socat")
	})

	containerRuntime := c.runtime()
	switch containerRuntime {

	case containerd.Name:
		r.Stage("installing " + containerRuntime + " dependencies")
		installContainerdDeps(c.host, c.guest, r)

	case docker.Name:
		// no known dependencies for now
	}

	// minikube
	r.Stage("installing minikube")
	installMinikube(c.host, c.guest, r, c.kubernetesVersion())

	// adding to chain to ensure it executes after successful provision
	r.Add(func() error {
		c.newlyProvisioned = true
		return nil
	})

	return r.Exec()
}

func (c kubernetesRuntime) Start() error {
	r := c.Init()
	if c.Running() {
		r.Println("already running")
		return nil
	}

	r.Stage("starting")

	r.Add(func() error {
		// first start takes time, it's better to inform the user
		if c.newlyProvisioned {
			r.Println("NOTE: this is the first startup of kubernetes, it will take a while")
			r.Println("      but no worries, subsequent startups only take some seconds")
		}

		args := []string{"minikube", "start",
			"--driver", "none",
			"--container-runtime", c.runtime(),
			"--kubernetes-version", c.kubernetesVersion(),
		}

		switch c.runtime() {
		case containerd.Name:
			args = append(args, "--cni", "bridge")
		}

		return c.guest.Run(args...)
	})

	if err := r.Exec(); err != nil {
		return err
	}

	return c.provisionKubeconfig()
}

func (c kubernetesRuntime) Stop() error {
	if c.runtime() == containerd.Name {
		// minikube stop with containerd runtime is ineffective at the moment.
		return nil
	}
	r := c.Init()
	r.Stage("stopping")
	r.Add(func() error {
		return c.guest.Run("minikube", "stop")
	})
	return r.Exec()
}

func (c kubernetesRuntime) Teardown() error {
	r := c.Init()
	r.Stage("deleting")
	r.Add(func() error {
		return c.guest.Run("minikube", "delete")
	})

	c.teardownKubeconfig(r)
	r.Add(func() error {
		return c.guest.Set(kubeconfigKey, "")
	})
	return r.Exec()
}

func (c kubernetesRuntime) Dependencies() []string {
	return []string{"kubectl"}
}

func (c kubernetesRuntime) Version() string {
	version, _ := c.host.RunOutput("kubectl", "--context", "colima", "version", "--short")
	return version
}

package kubernetes

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
)

// Name is container runtime name
const Name = "kubernetes"

func New(host environment.HostActions, guest environment.GuestActions, containerRuntime string) container.Container {
	return &kubernetesRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New("kubernetes"),
		runtime:      containerRuntime,
	}
}

var _ container.Container = (*kubernetesRuntime)(nil)

type kubernetesRuntime struct {
	host    environment.HostActions
	guest   environment.GuestActions
	runtime string
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

func (c kubernetesRuntime) Provision() error {
	r := c.Init()

	if c.isInstalled() {
		return nil
	}

	r.Stage("provisioning")
	deps := []string{"conntrack", "socat"}

	switch c.runtime {
	case containerd.Name:
		// minikube with containerd still needs docker :( https://github.com/kubernetes/minikube/issues/10908
		deps = append(deps, "docker.io")
	case docker.Name:
		// docker previously installed as part of docker provision
	}

	// apt install deps
	r.Add(func() error {
		args := append([]string{"sudo", "apt", "install", "-y"}, deps...)
		return c.guest.Run(args...)
	})
	r.Add(func() error {
		return c.guest.Run("curl", "-L", "-o", "/tmp/crictl.tar.gz")
	})
	// crictl
	return nil
}

func (c kubernetesRuntime) Start() error {
	r := c.Init()
	r.Stage("starting")
	r.Add(func() error {
		return c.guest.Run("minikube", "start", "--driver=none", "--container-runtime", c.runtime)
	})
	return r.Exec()
}

func (c kubernetesRuntime) Stop() error {
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
	return r.Exec()
}

func (c kubernetesRuntime) Dependencies() []string {
	return []string{"kubectl"}
}

func (c kubernetesRuntime) Version() string {
	version, _ := c.host.RunOutput("kubectl", "--context", "colima", "version", "--short")
	return version
}

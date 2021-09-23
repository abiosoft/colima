package kubernetes

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"os"
	"runtime"
)

// Name is container runtime name
const Name = "kubernetes"

func New(host environment.HostActions, guest environment.GuestActions, containerRuntime string) environment.Container {
	return &kubernetesRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New("kubernetes"),
		runtime:      containerRuntime,
	}
}

var _ environment.Container = (*kubernetesRuntime)(nil)

type kubernetesRuntime struct {
	host             environment.HostActions
	guest            environment.GuestActions
	runtime          string
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

func (c *kubernetesRuntime) Provision() error {
	r := c.Init()

	if c.isInstalled() {
		return nil
	}

	r.Stage("provisioning")

	// apt install deps
	r.Stage("installing dependencies")
	r.Add(func() error {
		// minikube with containerd still needs docker :( https://github.com/kubernetes/minikube/issues/10908
		return c.guest.Run("sudo", "apt", "install", "-y", "conntrack", "socat", "docker.io")
	})

	switch c.runtime {

	case containerd.Name:
		r.Stage("installing " + c.runtime + " dependencies")
		c.installCrictl(r)

	case docker.Name:
		// no known dependencies for now
	}

	// minikube
	c.installMinikube(r)

	// adding to chain to ensure it executes after successful provision
	r.Add(func() error {
		c.newlyProvisioned = true
		return nil
	})

	return r.Exec()
}

func (c kubernetesRuntime) installCrictl(r *cli.ActiveCommandChain) {
	version := "v1.22.0"
	downloadPath := "/tmp/crictl.tar.gz"
	url := "https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-${ARCH}.tar.gz"
	url = os.Expand(url, func(s string) string {
		switch s {
		case "VERSION":
			return version
		case "ARCH":
			return runtime.GOARCH
		}
		return ""
	})

	r.Add(func() error {
		return c.guest.Run("curl", "-L", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return c.guest.Run("sudo", "tar", "xvfz", downloadPath, "-C", "/usr/local/bin")
	})
}

func (c kubernetesRuntime) installMinikube(r *cli.ActiveCommandChain) {
	downloadPath := "/tmp/minikube"
	url := "https://storage.googleapis.com/minikube/releases/latest/minikube-linux-" + runtime.GOOS
	r.Add(func() error {
		return c.guest.Run("curl", "-L", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return c.guest.Run("sudo", "install", downloadPath, "/usr/local/bin/minikube")
	})
}

func (c kubernetesRuntime) Start() error {
	r := c.Init()
	r.Stage("starting")

	if c.newlyProvisioned {
		r.Println("NOTE: this is the first startup of kubernetes, it will take a while")
		r.Println("      but no worries, subsequent startups only take some seconds")
	}

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

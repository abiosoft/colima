package kubernetes

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Name is container runtime name
const Name = "kubernetes"

// New creates a new kubernetes runtime.
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
	r.Stage("installing minikube")
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
		return c.guest.RunInteractive("curl", "-L", "-#", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return c.guest.Run("sudo", "tar", "xvfz", downloadPath, "-C", "/usr/local/bin")
	})
}

func (c kubernetesRuntime) installMinikube(r *cli.ActiveCommandChain) {
	downloadPath := "/tmp/minikube"
	url := "https://storage.googleapis.com/minikube/releases/latest/minikube-linux-" + runtime.GOOS
	r.Add(func() error {
		return c.guest.RunInteractive("curl", "-L", "-#", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return c.guest.Run("sudo", "install", downloadPath, "/usr/local/bin/minikube")
	})
}

func (c kubernetesRuntime) Start() error {
	r := c.Init()
	r.Stage("starting")

	r.Add(func() error {
		// first start takes time, it's better to inform the user
		if c.newlyProvisioned {
			r.Println("NOTE: this is the first startup of kubernetes, it will take a while")
			r.Println("      but no worries, subsequent startups only take some seconds")
		}
		return c.guest.Run("minikube", "start", "--driver", "none", "--container-runtime", c.runtime)
	})

	if err := r.Exec(); err != nil {
		return err
	}

	return c.provisionKubeconfig()
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

const kubeconfigKey = "kubeconfig"

func (c kubernetesRuntime) provisionKubeconfig() error {
	provisioned, _ := strconv.ParseBool(c.guest.Get(kubeconfigKey))
	if provisioned {
		return nil
	}

	r := c.Init()

	r.Stage("updating " + Name + " configuration")

	// ensure host kube directory exists
	hostHome := c.host.Env("HOME")
	if hostHome == "" {
		return fmt.Errorf("error retrieving home directory on host")
	}

	hostKubeDir := filepath.Join(hostHome, ".kube")
	r.Add(func() error {
		return c.host.Run("mkdir", "-p", hostKubeDir)
	})

	kubeconfFile := filepath.Join(hostKubeDir, "config")
	tmpkubeconfFile := filepath.Join(hostKubeDir, "colima-temp")

	// manipulate in VM and save to host
	r.Add(func() error {
		// flatten in lima for portability
		kubeconfig, err := c.guest.RunOutput("minikube", "kubectl", "--", "config", "view", "--flatten")
		if err != nil {
			return err
		}
		// replace unreachable ip with localhost
		kubeconfig = strings.ReplaceAll(kubeconfig, "192.168.5.15:8443", "127.0.0.1:8443")
		// rename to $NAME
		kubeconfig = strings.ReplaceAll(kubeconfig, "minikube", config.AppName())
		// reverse unintended rename
		kubeconfig = strings.ReplaceAll(kubeconfig, config.AppName()+".sigs.k8s.io", "minikube.sigs.k8s.io")

		// save on the host
		return c.host.Write(tmpkubeconfFile, kubeconfig)
	})

	// merge on host
	r.Add(func() (err error) {
		// prepare new host with right env var.
		envVar := fmt.Sprintf("KUBECONFIG=%s:%s", kubeconfFile, tmpkubeconfFile)
		host := c.host.WithEnv([]string{envVar})

		// get merged config
		kubeconfig, err := host.RunOutput("kubectl", "view", "--raw")
		if err != nil {
			return err
		}

		// save
		return host.Write(tmpkubeconfFile, kubeconfig)
	})

	// backup current settings and save new config
	r.Add(func() error {
		// backup existing file if exists
		if stat, err := c.host.Stat(kubeconfFile); err == nil && !stat.IsDir() {
			backup := filepath.Join(filepath.Dir(kubeconfFile), fmt.Sprintf("config-bak-%d", time.Now().Unix()))
			if err := c.host.Run("cp", kubeconfFile, backup); err != nil {
				return fmt.Errorf("error backing up kubeconfig: %w", err)
			}
		}
		// save new config
		if err := c.host.Run("cp", tmpkubeconfFile, kubeconfFile); err != nil {
			return fmt.Errorf("error updating kubeconfig: %w", err)
		}

		return nil
	})

	// set new context
	r.Add(func() error {
		return c.host.RunInteractive("kubectl", "config", "use-context", config.AppName())
	})

	// save settings
	r.Add(func() error {
		return c.guest.Set(kubeconfigKey, "true")
	})

	return r.Exec()
}

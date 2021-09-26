package kubernetes

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const kubeconfigKey = "kubeconfig"

func (c kubernetesRuntime) provisionKubeconfig() error {
	provisioned, _ := strconv.ParseBool(c.guest.Get(kubeconfigKey))
	if provisioned {
		return nil
	}

	r := c.Init()

	r.Stage("updating kubeconfig")

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
		host := c.host.WithEnv(envVar)

		// get merged config
		kubeconfig, err := host.RunOutput("kubectl", "config", "view", "--raw")
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
func (c kubernetesRuntime) teardownKubeconfig(r *cli.ActiveCommandChain) {
	r.Stage("reverting kubeconfig")

	r.Add(func() error {
		return c.host.Run("kubectl", "config", "unset", "users."+config.AppName())
	})
	r.Add(func() error {
		return c.host.Run("kubectl", "config", "unset", "contexts."+config.AppName())
	})
	r.Add(func() error {
		return c.host.Run("kubectl", "config", "unset", "clusters."+config.AppName())
	})
}

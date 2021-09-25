package kubernetes

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"os"
	"runtime"
)

func (c *kubernetesRuntime) containerdDeps(r *cli.ActiveCommandChain) {
	// install crictl
	c.installCrictl(r)

	// minikube with containerd still needs docker :( https://github.com/kubernetes/minikube/issues/10908
	// the good news is we can spoof it.

	// write fake docker file
	r.Add(func() error {
		return c.guest.Run("sudo", "sh", "-c", `printf "#!/usr/bin/env bash\nexit 0" > /usr/local/bin/docker`)
	})
	// make it executable
	r.Add(func() error {
		return c.guest.Run("sudo", "chmod", "+x", "/usr/local/bin/docker")
	})

	// fix cni permission
	r.Add(func() error {
		user, err := c.guest.User()
		if err != nil {
			return fmt.Errorf("error retrieving username: %w", err)
		}
		return c.guest.Run("sudo", "chown", "-R", user+":"+user, "/etc/cni")
	})
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

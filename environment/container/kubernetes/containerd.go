package kubernetes

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/downloader"
	"os"
	"path/filepath"
	"runtime"
)

func installContainerdDeps(host environment.HostActions, guest environment.GuestActions, r *cli.ActiveCommandChain) {
	// install crictl
	installCrictl(host, guest, r)

	// minikube with containerd still needs docker :( https://github.com/kubernetes/minikube/issues/10908
	// the good news is we can spoof it.

	// write fake docker file
	r.Add(func() error {
		return guest.Run("sudo", "sh", "-c", `printf "#!/usr/bin/env bash\nexit 0" > /usr/local/bin/docker`)
	})
	// make it executable
	r.Add(func() error {
		return guest.Run("sudo", "chmod", "+x", "/usr/local/bin/docker")
	})

	// fix cni permission
	r.Add(func() error {
		user, err := guest.User()
		if err != nil {
			return fmt.Errorf("error retrieving username: %w", err)
		}
		return guest.Run("sudo", "chown", "-R", user+":"+user, "/etc/cni")
	})
	// fix cni path
	r.Add(func() error {
		cniDir := "/opt/cni/bin"
		if err := guest.Run("ls", cniDir); err == nil {
			return nil
		}

		if err := guest.Run("sudo", "mkdir", "-p", filepath.Dir(cniDir)); err != nil {
			return err
		}
		return guest.Run("sudo", "ln", "-s", "/usr/local/libexec/cni", cniDir)
	})
}

func installCrictl(host environment.Host, guest environment.GuestActions, r *cli.ActiveCommandChain) {
	// TODO figure a way to keep up to date.
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
		return downloader.Download(host, guest, url, downloadPath)
	})
	r.Add(func() error {
		return guest.Run("sudo", "tar", "xvfz", downloadPath, "-C", "/usr/local/bin")
	})
}

package kubernetes

import (
	_ "embed"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/util/downloader"
	"strings"
)

const k3sVersion = "v1.22.2+k3s2"

func installK3s(host environment.HostActions, guest environment.GuestActions, a *cli.ActiveCommandChain, containerRuntime string) {
	installK3sBinary(host, guest, a)
	installK3sCache(host, guest, a, containerRuntime)
	installK3sCluster(host, guest, a, containerRuntime)

	if containerRuntime == containerd.Name {
		installContainerdDeps(guest, a)
	}
}

func installK3sBinary(host environment.HostActions, guest environment.GuestActions, a *cli.ActiveCommandChain) {
	// install k3s last to ensure it is the last step
	downloadPath := "/tmp/k3s"
	url := "https://github.com/k3s-io/k3s/releases/download/" + k3sVersion + "/k3s"
	if guest.Arch().GoArch() == "arm64" {
		url += "-arm64"
	}
	a.Add(func() error {
		return downloader.Download(host, guest, url, downloadPath)
	})
	a.Add(func() error {
		return guest.Run("sudo", "install", downloadPath, "/usr/local/bin/k3s")
	})
}

func installK3sCache(host environment.HostActions, guest environment.GuestActions, a *cli.ActiveCommandChain, containerRuntime string) {
	imageTar := "k3s-airgap-images-" + guest.Arch().GoArch() + ".tar"
	imageTarGz := imageTar + ".gz"
	downloadPathTar := "/tmp/" + imageTar
	downloadPathTarGz := "/tmp/" + imageTarGz
	url := "https://github.com/k3s-io/k3s/releases/download/" + k3sVersion + "/" + imageTarGz
	a.Add(func() error {
		return downloader.Download(host, guest, url, downloadPathTarGz)
	})
	a.Add(func() error {
		return guest.Run("gzip", "-f", "-d", downloadPathTarGz)
	})

	airGapDir := "/var/lib/rancher/k3s/agent/images/"
	a.Add(func() error {
		return guest.Run("sudo", "mkdir", "-p", airGapDir)
	})
	a.Add(func() error {
		return guest.Run("sudo", "cp", downloadPathTar, airGapDir)
	})

	switch containerRuntime {
	case containerd.Name:
		a.Stage("loading containerd images")
		a.Add(func() error {
			return guest.Run("sudo", "ctr", "-n", "k8s.io", "images", "import", downloadPathTar)
		})
	case docker.Name:
		a.Stage("loading docker images")
		a.Add(func() error {
			return guest.Run("sudo", "docker", "load", "-i", downloadPathTar)
		})
	}

}

func installK3sCluster(host environment.HostActions, guest environment.GuestActions, a *cli.ActiveCommandChain, containerRuntime string) {
	// install k3s last to ensure it is the last step
	downloadPath := "/tmp/k3s-install.sh"
	url := "https://raw.githubusercontent.com/k3s-io/k3s/" + k3sVersion + "/install.sh"
	a.Add(func() error {
		return downloader.Download(host, guest, url, downloadPath)
	})
	a.Add(func() error {
		return guest.Run("sudo", "install", downloadPath, "/usr/local/bin/k3s-install.sh")
	})

	args := []string{
		"--write-kubeconfig-mode", "644",
		"--resolv-conf", "/run/systemd/resolve/resolv.conf",
		"--disable", "traefik",
	}

	switch containerRuntime {
	case docker.Name:
		args = append(args, "--docker")
	case containerd.Name:
		args = append(args, "--container-runtime-endpoint", "unix:///run/containerd/containerd.sock")
	}
	a.Add(func() error {
		return guest.Run("sh", "-c", "INSTALL_K3S_SKIP_DOWNLOAD=true INSTALL_K3S_SKIP_ENABLE=true k3s-install.sh "+strings.Join(args, " "))
	})

}

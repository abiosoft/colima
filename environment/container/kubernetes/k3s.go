package kubernetes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/downloader"
	"github.com/sirupsen/logrus"
)

const listenPortKey = "k3s_listen_port"

func installK3s(host environment.HostActions,
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	log *logrus.Entry,
	containerRuntime string,
	k3sVersion string,
	disable []string,
) {
	installK3sBinary(host, guest, a, k3sVersion)
	installK3sCache(host, guest, a, log, containerRuntime, k3sVersion)
	installK3sCluster(host, guest, a, containerRuntime, k3sVersion, disable)
}

func installK3sBinary(
	host environment.HostActions,
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	k3sVersion string,
) {
	downloadPath := "/tmp/k3s"

	baseURL := "https://github.com/k3s-io/k3s/releases/download/" + k3sVersion + "/"
	shaSumTxt := "sha256sum-" + guest.Arch().GoArch() + ".txt"

	url := baseURL + "k3s"
	shaURL := baseURL + shaSumTxt
	if guest.Arch().GoArch() == "arm64" {
		url += "-arm64"
	}
	a.Add(func() error {
		r := downloader.Request{
			URL: url,
			SHA: &downloader.SHA{Size: 256, URL: shaURL},
		}
		return downloader.DownloadToGuest(host, guest, r, downloadPath)
	})
	a.Add(func() error {
		return guest.Run("sudo", "install", downloadPath, "/usr/local/bin/k3s")
	})
}

func installK3sCache(
	host environment.HostActions,
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	log *logrus.Entry,
	containerRuntime string,
	k3sVersion string,
) {
	baseURL := "https://github.com/k3s-io/k3s/releases/download/" + k3sVersion + "/"
	imageTar := "k3s-airgap-images-" + guest.Arch().GoArch() + ".tar"
	shaSumTxt := "sha256sum-" + guest.Arch().GoArch() + ".txt"
	imageTarGz := imageTar + ".gz"
	downloadPathTar := "/tmp/" + imageTar
	downloadPathTarGz := "/tmp/" + imageTarGz
	url := baseURL + imageTarGz
	shaURL := baseURL + shaSumTxt
	a.Add(func() error {
		r := downloader.Request{
			URL: url,
			SHA: &downloader.SHA{Size: 256, URL: shaURL},
		}
		return downloader.DownloadToGuest(host, guest, r, downloadPathTarGz)
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

	// load OCI images for K3s
	// this can be safely ignored if failed as the images would be pulled afterwards.
	switch containerRuntime {
	case containerd.Name:
		a.Stage("loading oci images")
		a.Add(func() error {
			if err := guest.Run("sudo", "nerdctl", "-n", "k8s.io", "load", "-i", downloadPathTar, "--all-platforms"); err != nil {
				log.Warnln(fmt.Errorf("error loading oci images: %w", err))
				log.Warnln("startup may delay a bit as images will be pulled from oci registry")
			}
			return nil
		})
	case docker.Name:
		a.Stage("loading oci images")
		a.Add(func() error {
			if err := guest.Run("sudo", "docker", "load", "-i", downloadPathTar); err != nil {
				log.Warnln(fmt.Errorf("error loading oci images: %w", err))
				log.Warnln("startup may delay a bit as images will be pulled from oci registry")
			}
			return nil
		})
	}
}

func installK3sCluster(
	host environment.HostActions,
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	containerRuntime string,
	k3sVersion string,
	k3sArgs []string,
) {
	// install k3s last to ensure it is the last step
	downloadPath := "/tmp/k3s-install.sh"
	url := "https://raw.githubusercontent.com/k3s-io/k3s/" + k3sVersion + "/install.sh"
	a.Add(func() error {
		r := downloader.Request{URL: url}
		return downloader.DownloadToGuest(host, guest, r, downloadPath)
	})
	a.Add(func() error {
		return guest.Run("sudo", "install", downloadPath, "/usr/local/bin/k3s-install.sh")
	})

	args := append([]string{
		"--write-kubeconfig-mode", "644",
	}, k3sArgs...)

	// replace ip address if networking is enabled
	ipAddress := limautil.IPAddress(config.CurrentProfile().ID)
	if ipAddress == "127.0.0.1" {
		args = append(args, "--flannel-iface", "eth0")
	} else {
		args = append(args, "--advertise-address", ipAddress)
		args = append(args, "--flannel-iface", limautil.NetInterface)
	}

	switch containerRuntime {
	case docker.Name:
		args = append(args, "--docker")
	case containerd.Name:
		args = append(args, "--container-runtime-endpoint", "unix:///run/containerd/containerd.sock")
	}

	a.Add(func() error {
		port, err := getPortNumber(guest)
		if err != nil {
			return err
		}
		args = append(args, "--https-listen-port", strconv.Itoa(port))
		return nil
	})

	a.Add(func() error {
		return guest.Run("sh", "-c", "INSTALL_K3S_SKIP_DOWNLOAD=true INSTALL_K3S_SKIP_ENABLE=true k3s-install.sh "+strings.Join(args, " "))
	})
}

// getPortNumber retrieves the previously set port number.
// If missing, an available random port is set and return.
func getPortNumber(guest environment.GuestActions) (int, error) {
	// port previously set, reuse it
	if port, err := strconv.Atoi(guest.Get(listenPortKey)); err == nil && port > 0 {
		return port, nil
	}

	// for backward compatibility
	// if the instance already exists, assume default port 6443
	if m := guest.Get(masterAddressKey); m != "" {
		return 6443, nil
	}

	// new instance, assign random port
	port := util.RandomAvailablePort()
	if err := guest.Set(listenPortKey, strconv.Itoa(port)); err != nil {
		return 0, err
	}

	return port, nil
}

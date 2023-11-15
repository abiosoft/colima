package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/downloader"
	"github.com/coreos/go-semver/semver"
)

const (
	version     = "v0.6.0-2" // version of colima-core to use.
	limaVersion = "v0.18.0"  // minimum Lima version supported
	baseURL     = "https://github.com/abiosoft/colima-core/releases/download/" + version + "/"
)

type (
	hostActions  = environment.HostActions
	guestActions = environment.GuestActions
)

func downloadSha(url string) *downloader.SHA {
	return &downloader.SHA{
		Size: 512,
		URL:  url + ".sha512sum",
	}
}

// SetupBinfmt downloads and install binfmt
func SetupBinfmt(host hostActions, guest guestActions, arch environment.Arch) error {
	qemuArch := environment.AARCH64
	if arch.Value().GoArch() == "arm64" {
		qemuArch = environment.X8664
	}

	install := func() error {
		if err := guest.Run("sh", "-c", "sudo QEMU_PRESERVE_ARGV0=1 /usr/bin/binfmt --install "+qemuArch.GoArch()); err != nil {
			return fmt.Errorf("error installing binfmt: %w", err)
		}
		return nil
	}

	// ignore download and extract if previously installed
	if err := guest.RunQuiet("command", "-v", "binfmt"); err == nil {
		return install()
	}

	// download
	url := baseURL + "binfmt-" + arch.Value().GoArch() + ".tar.gz"
	dest := "/tmp/binfmt.tar.gz"
	if err := downloader.Download(host, guest, downloader.Request{
		URL:      url,
		SHA:      downloadSha(url),
		Filename: dest,
	}); err != nil {
		return fmt.Errorf("error downloading binfmt: %w", err)
	}

	// extract
	if err := guest.Run("sh", "-c",
		strings.NewReplacer(
			"{file}", dest,
			"{qemu_arch}", string(qemuArch),
		).Replace(`cd /tmp && tar xfz {file} && sudo chown root:root binfmt qemu-{qemu_arch} && sudo mv binfmt qemu-{qemu_arch} /usr/bin`),
	); err != nil {
		return fmt.Errorf("error extracting binfmt: %w", err)
	}

	return install()
}

// SetupContainerdUtils downloads and install containerd utils.
func SetupContainerdUtils(host hostActions, guest guestActions, arch environment.Arch) error {
	// ignore if already installed
	if err := guest.RunQuiet("sh", "-c", "command -v nerdctl && stat /opt/cni/bin/flannel"); err == nil {
		return nil
	}

	// download
	url := baseURL + "containerd-utils-" + arch.Value().GoArch() + ".tar.gz"
	dest := "/tmp/containerd-utils.tar.gz"
	if err := downloader.Download(host, guest, downloader.Request{
		URL:      url,
		SHA:      downloadSha(url),
		Filename: dest,
	}); err != nil {
		return fmt.Errorf("error downloading containerd-utils: %w", err)
	}

	// extract
	if err := guest.Run("sh", "-c",
		strings.NewReplacer(
			"{archive}", dest,
		).Replace(`cd /tmp && sudo tar Cxfz /usr/local {archive} && sudo mkdir -p /opt/cni && sudo mv /usr/local/libexec/cni /opt/cni/bin`),
	); err != nil {
		return fmt.Errorf("error extracting containerd utils: %w", err)
	}

	return nil
}

// LimaVersionSupported checks if the currently installed Lima version is supported.
func LimaVersionSupported() error {
	var values struct {
		Version string `json:"version"`
	}
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "info")
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error checking Lima version: %w", err)
	}

	if err := json.NewDecoder(&buf).Decode(&values); err != nil {
		return fmt.Errorf("error decoding 'limactl info' json: %w", err)
	}
	// remove pre-release hyphen
	parts := strings.SplitN(values.Version, "-", 2)
	if len(parts) > 0 {
		values.Version = parts[0]
	}

	if parts[0] == "HEAD" {
		logrus.Warnf("to avoid compatibility issues, ensure lima development version (%s) in use is not lower than %s", values.Version, limaVersion)
		return nil
	}

	min := semver.New(strings.TrimPrefix(limaVersion, "v"))
	current, err := semver.NewVersion(strings.TrimPrefix(values.Version, "v"))
	if err != nil {
		return fmt.Errorf("invalid semver version for Lima: %w", err)
	}

	if min.Compare(*current) > 0 {
		return fmt.Errorf("minimum Lima version supported is %s, current version is %s", limaVersion, values.Version)
	}

	return nil
}

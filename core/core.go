package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/coreos/go-semver/semver"
)

const limaVersion = "v0.18.0" // minimum Lima version supported

type (
	hostActions  = environment.HostActions
	guestActions = environment.GuestActions
)

// SetupBinfmt downloads and install binfmt
func SetupBinfmt(host hostActions, guest guestActions, arch environment.Arch) error {
	qemuArch := environment.AARCH64
	if arch.Value().GoArch() == "arm64" {
		qemuArch = environment.X8664
	}

	install := func() error {
		if err := guest.Run("sh", "-c", "sudo QEMU_PRESERVE_ARGV0=1 /usr/bin/binfmt --install 386,"+qemuArch.GoArch()); err != nil {
			return fmt.Errorf("error installing binfmt: %w", err)
		}
		return nil
	}

	// validate binfmt
	if err := guest.RunQuiet("command", "-v", "binfmt"); err != nil {
		return fmt.Errorf("binfmt not found: %w", err)
	}

	return install()
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

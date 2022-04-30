package gvproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon"
)

var _ daemon.Dependency = qemuBinsSymlinks{}

// only these two are required for Lima
var qemuBins = []string{"qemu-system-aarch64", "qemu-system-x86_64"}

type qemuBinsSymlinks struct{}

func (q qemuBinsSymlinks) dir() string { return filepath.Join(config.WrapperDir(), "bin") }

func (q qemuBinsSymlinks) Installed() bool {
	for _, bin := range qemuBins {
		bin = filepath.Join(q.dir(), bin)
		if _, err := os.Stat(bin); err != nil {
			logrus.Info("error stat: %w", err)
			return false
		}
	}

	return true
}

func (q qemuBinsSymlinks) Install(host environment.HostActions) error {
	dir := q.dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error preparing qemu wrapper bin directory: %w", err)
	}
	this, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot retrieve current process: %w", err)
	}

	for _, bin := range qemuBins {
		bin = filepath.Join(q.dir(), bin)
		if err := host.Run("ln", "-sf", this, bin); err != nil {
			return fmt.Errorf("error wrapping %s: %w", bin, err)
		}
	}

	return nil
}

var _ daemon.Dependency = qemuShareDirSymlink{}

type qemuShareDirSymlink struct{}

func (q qemuShareDirSymlink) dir() string { return filepath.Join(config.WrapperDir(), "share") }

func (q qemuShareDirSymlink) Installed() bool {
	dir := q.dir()
	if _, err := os.Stat(dir); err != nil {
		logrus.Infof("error stat: %v", err)
		return false
	}
	return true
}

func (q qemuShareDirSymlink) Install(host environment.HostActions) error {
	dir := q.dir()
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("error preparing qemu wrapper shared directory: %w", err)
	}

	qemu, err := exec.LookPath("qemu-img")
	if err != nil {
		return fmt.Errorf("error locating qemu binaries in PATH: %w", err)
	}
	qemuBinDir := filepath.Dir(qemu)
	if !strings.HasSuffix(qemuBinDir, "/bin") {
		return fmt.Errorf("unsupport bin directory '%s' for qemu", qemuBinDir)
	}
	qemuShareDir := filepath.Join(filepath.Dir(qemuBinDir), "share")

	if err := host.Run("ln", "-sf", qemuShareDir, dir); err != nil {
		return fmt.Errorf("error wrapping qemu share directory: %w", err)
	}

	return nil
}

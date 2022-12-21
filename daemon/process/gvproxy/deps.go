package gvproxy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/qemu"
	"github.com/abiosoft/colima/util/fsutil"
)

var _ process.Dependency = qemuBinsSymlinks{}

type qemuBinsSymlinks struct{}

func (q qemuBinsSymlinks) Installed() bool {
	dir := qemu.LimaDir()
	for _, bin := range qemu.Binaries {
		bin = filepath.Join(dir.Bin(), bin)
		if _, err := os.Stat(bin); err != nil {
			return false
		}
	}

	return true
}

func (q qemuBinsSymlinks) Install(host environment.HostActions) error {
	dir := qemu.LimaDir()
	if err := fsutil.MkdirAll(dir.Bin(), 0755); err != nil {
		return fmt.Errorf("error preparing qemu wrapper bin directory: %w", err)
	}
	this, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot retrieve current process: %w", err)
	}

	for _, bin := range qemu.Binaries {
		bin = filepath.Join(dir.Bin(), bin)
		if err := host.Run("ln", "-sf", this, bin); err != nil {
			return fmt.Errorf("error wrapping %s: %w", bin, err)
		}
	}

	return nil
}

var _ process.Dependency = qemuShareDirSymlink{}

type qemuShareDirSymlink struct{}

func (q qemuShareDirSymlink) Installed() bool {
	dir := qemu.LimaDir()
	if _, err := os.Stat(dir.Share()); err != nil {
		return false
	}
	return true
}

func (q qemuShareDirSymlink) Install(host environment.HostActions) error {
	limaDir := qemu.LimaDir()
	if err := fsutil.MkdirAll(limaDir.Root(), 0755); err != nil {
		return fmt.Errorf("error preparing qemu wrapper shared directory: %w", err)
	}

	hostDir, err := qemu.HostDir()
	if err != nil {
		return fmt.Errorf("error retrieving qemu installation location: %w", err)
	}
	if err := host.Run("ln", "-sf", hostDir.Share(), limaDir.Share()); err != nil {
		return fmt.Errorf("error wrapping qemu share directory: %w", err)
	}

	return nil
}

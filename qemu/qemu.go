package qemu

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

// Qemu binaries
const (
	BinAARCH64 = "qemu-system-aarch64"
	BinX8664   = "qemu-system-x86_64"
)

// Binaries is a list of Qemu binaries
var Binaries = []string{
	BinAARCH64,
	BinX8664,
}

// InstallDir is a typical Unix installation directory that contains `bin` and `share`.
type InstallDir string

// Bin is the directory for Qemu binaries.
// Typically what gets added to PATH.
func (i InstallDir) Bin() string {
	return filepath.Join(string(i), "bin")
}

// Share is the corresponding share directory for BinDir.
func (i InstallDir) Share() string {
	return filepath.Join(string(i), "share")
}

// Root points to this InstallDir.
func (i InstallDir) Root() string {
	return string(i)
}

// BinsEnvVar returns the environment variables for the Qemu binaries.
//  QEMU_SYSTEM_X86_64=/path/to/x86-bin
//  QEMU_SYSTEM_AARCH64=/path/to/aarch64-bin
func (i InstallDir) BinsEnvVar() []string {
	return []string{
		"QEMU_SYSTEM_X86_64=" + filepath.Join(i.Bin(), BinX8664),
		"QEMU_SYSTEM_AARCH64=" + filepath.Join(i.Bin(), BinAARCH64),
	}
}

// HostDir returns the install directory for Qemu on the host.
func HostDir() (InstallDir, error) {
	qemu, err := exec.LookPath("qemu-system-" + string(environment.HostArch().Value()))
	if err != nil {
		return "", fmt.Errorf("error locating qemu binaries in PATH: %w", err)
	}
	qemuBinDir := filepath.Dir(qemu)
	if !strings.HasSuffix(qemuBinDir, "/bin") {
		return "", fmt.Errorf("unsupport bin directory '%s' for qemu", qemuBinDir)
	}

	return InstallDir(filepath.Dir(qemuBinDir)), nil
}

// LimaDir returns the install directory for Qemu to be utilised by Lima.
func LimaDir() InstallDir {
	return InstallDir(config.WrapperDir())
}

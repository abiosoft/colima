package environment

import (
	"context"
	"runtime"

	"github.com/abiosoft/colima/util"
)

// VM is virtual machine.
type VM interface {
	GuestActions
	Dependencies
	Host() HostActions
	Teardown(ctx context.Context) error
}

// VM configurations
const (
	// ContainerRuntimeKey is the settings key for container runtime.
	ContainerRuntimeKey = "runtime"
)

// Arch is the CPU architecture of the VM.
type Arch string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
)

// HostArch returns the host CPU architecture.
func HostArch() Arch {
	return Arch(runtime.GOARCH).Value()
}

// GoArch returns the GOARCH equivalent value for the architecture.
func (a Arch) GoArch() string {
	switch a {
	case X8664:
		return "amd64"
	case AARCH64:
		return "arm64"
	}

	return runtime.GOARCH
}

// Value converts the underlying architecture alias value to one of X8664 or AARCH64.
func (a Arch) Value() Arch {
	switch a {
	case X8664, AARCH64:
		return a
	// accept amd, amd64, x86, x64, arm, arm64 and m1 values
	case "amd", "amd64", "x86", "x64":
		return X8664
	case "arm", "arm64", "m1":
		return AARCH64
	}

	return Arch(runtime.GOARCH).Value()
}

// DefaultVMType returns the default virtual machine type based on the operation
// system and availability of Qemu.
func DefaultVMType() string {
	if util.MacOS13OrNewer() {
		return "vz"
	}

	return "qemu"
}

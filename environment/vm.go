package environment

import "runtime"

// VM is virtual machine.
type VM interface {
	GuestActions
	Dependencies
	Host() HostActions
	Teardown() error
}

// VM configurations
const (
	// ContainerRuntimeKey is the settings key for container runtime.
	ContainerRuntimeKey = "runtime"
	// KubernetesVersionKey is the settings key for kubernetes version.
	KubernetesVersionKey = "kubernetes_version"
	// SSHPortKey is the settings for the VM SSH port.
	SSHPortKey = "ssh_port"
)

// Arch is the VM architecture.
type Arch string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
)

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

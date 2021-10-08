package environment

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

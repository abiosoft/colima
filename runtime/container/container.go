package container

import (
	"github.com/abiosoft/colima/runtime"
)

// Container is container runtime.
type Container interface {
	// Name is the name of the container runtime. e.g. docker, containerd
	Name() string
	// Provision provisions/installs the container runtime.
	// Should be idempotent.
	Provision() error
	// Start starts the container runtime.
	Start() error
	// Stop stops the container runtime.
	Stop() error
	// Teardown tears down/uninstall the container runtime.
	Teardown() error

	runtime.Dependencies
}

type Runtime string

const (
	Docker     Runtime = "docker"
	ContainerD Runtime = "containerd"
)

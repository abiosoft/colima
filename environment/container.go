package environment

import (
	"fmt"
	"log"
)

// Container is container environment.
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
	// Version returns the container runtime version.
	Version() string

	Dependencies
}

// NewContainer creates a new container environment.
func NewContainer(runtime string, host HostActions, guest GuestActions) (Container, error) {
	if _, ok := containerRuntimes[runtime]; !ok {
		return nil, fmt.Errorf("invalid container runtime '%s'", runtime)
	}

	return containerRuntimes[runtime](host, guest), nil
}

// NewContainerFunc is implemented by container runtime implementations to create a new instance.
type NewContainerFunc func(host HostActions, guest GuestActions) Container

var containerRuntimes = map[string]NewContainerFunc{}

// RegisterContainer registers a new container runtime.
func RegisterContainer(name string, f NewContainerFunc) {
	if _, ok := containerRuntimes[name]; ok {
		log.Fatalf("container runtime '%s' already registered", name)
	}
	containerRuntimes[name] = f
}

// ContainerRuntimes return the names of available container runtimes.
func ContainerRuntimes() (names []string) {
	for name := range containerRuntimes {
		names = append(names, name)
	}
	return
}

// ContainerRuntimeKey is the settings key for container runtime.
const ContainerRuntimeKey = "runtime"

package environment

import (
	"context"
	"fmt"
	"log"
)

// Container is container environment.
type Container interface {
	// Name is the name of the container runtime. e.g. docker, containerd
	Name() string
	// Provision provisions/installs the container runtime.
	// Should be idempotent.
	Provision(ctx context.Context) error
	// Start starts the container runtime.
	Start(ctx context.Context) error
	// Stop stops the container runtime.
	Stop(ctx context.Context) error
	// Teardown tears down/uninstall the container runtime.
	Teardown(ctx context.Context) error
	// Version returns the container runtime version.
	Version() string
	// Running returns if the container runtime is currently running.
	Running() bool

	Dependencies
}

// NewContainer creates a new container environment.
func NewContainer(runtime string, host HostActions, guest GuestActions) (Container, error) {
	if _, ok := containerRuntimes[runtime]; !ok {
		return nil, fmt.Errorf("unsupported container runtime '%s'", runtime)
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
		// exclude kubernetes from the runtime list
		// TODO find a cleaner way to not hardcode kubernetes
		if name == "kubernetes" {
			continue
		}
		names = append(names, name)
	}
	return
}

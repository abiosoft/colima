package container

import (
	"fmt"
	"github.com/abiosoft/colima/runtime"
	"log"
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
	// Version returns the container runtime version.
	Version() string

	runtime.Dependencies
}

// New creates a new container runtime. `name` must be a valid container runtime name.
func New(name string, host runtime.HostActions, guest runtime.GuestActions) (Container, error) {
	if _, ok := runtimes[name]; !ok {
		return nil, fmt.Errorf("invalid container runtime '%s'", name)
	}

	return runtimes[name](host, guest), nil
}

// NewFunc is implemented by container runtime implementations to create a new instance.
type NewFunc func(host runtime.HostActions, guest runtime.GuestActions) Container

var runtimes = map[string]NewFunc{}

// Register registers a new runtime.
func Register(name string, f NewFunc) {
	if _, ok := runtimes[name]; ok {
		log.Fatalf("container runtime '%s' already registered", name)
	}
	runtimes[name] = f
}

// Names return the names of available container runtimes.
func Names() (names []string) {
	for name := range runtimes {
		names = append(names, name)
	}
	return
}

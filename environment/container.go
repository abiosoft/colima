package environment

import (
	"context"
	"fmt"
	"log"
)

// IsNoneRuntime returns if runtime is none.
func IsNoneRuntime(runtime string) bool { return runtime == "none" }

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
	// Update the container runtime.
	Update(ctx context.Context) (bool, error)
	// Version returns the container runtime version.
	Version(ctx context.Context) string
	// Running returns if the container runtime is currently running.
	Running(ctx context.Context) bool

	Dependencies
}

// NewContainer creates a new container environment.
func NewContainer(runtime string, host HostActions, guest GuestActions) (Container, error) {
	if _, ok := containerRuntimes[runtime]; !ok {
		return nil, fmt.Errorf("unsupported container runtime '%s'", runtime)
	}

	return containerRuntimes[runtime].Func(host, guest), nil
}

// NewContainerFunc is implemented by container runtime implementations to create a new instance.
type NewContainerFunc func(host HostActions, guest GuestActions) Container

var containerRuntimes = map[string]containerRuntimeFunc{}

type containerRuntimeFunc struct {
	Func   NewContainerFunc
	Hidden bool
}

// RegisterContainer registers a new container runtime.
// If hidden is true, the container is not displayed as an available runtime.
func RegisterContainer(name string, f NewContainerFunc, hidden bool) {
	if _, ok := containerRuntimes[name]; ok {
		log.Fatalf("container runtime '%s' already registered", name)
	}
	containerRuntimes[name] = containerRuntimeFunc{Func: f, Hidden: hidden}
}

// ContainerRuntimes return the names of available container runtimes.
func ContainerRuntimes() (names []string) {
	for name, cont := range containerRuntimes {
		if cont.Hidden {
			continue
		}
		names = append(names, name)
	}
	return
}

// UpdateInfo describes available updates for a container runtime.
type UpdateInfo struct {
	// Available indicates if updates are available.
	Available bool
	// Description is a human-readable summary of the available updates.
	Description string
}

// AppUpdater is an optional interface for container runtimes that require
// app-level control during updates (e.g., stop and restart).
// Container runtimes that implement this interface will use the multi-step
// update flow instead of the standard Container.Update() method.
type AppUpdater interface {
	// CheckUpdate checks for available updates.
	CheckUpdate(ctx context.Context) (UpdateInfo, error)
	// DownloadUpdate downloads the update packages.
	// Called before the instance is stopped.
	DownloadUpdate(ctx context.Context) error
	// InstallUpdate installs the previously downloaded update packages.
	// Called after the instance is stopped.
	InstallUpdate(ctx context.Context) error
}

// DataDisk holds the configuration for mounting an external runtime disk.
type DataDisk struct {
	Dirs     []DiskDir // the directories to be mounted
	PreMount []string  // the scripts to run before mounting the directories
	FSType   string    // the filesystem type for the disk e.g. ext4
}

// DiskDir is a directory mounted in a data disk.
type DiskDir struct {
	Name string
	Path string
}

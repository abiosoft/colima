// Package cruntime is container runtime package
package cruntime

// Runtime is container runtime.
type Runtime interface {
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
	// HostDependencies are dependencies that must exist on the host.
	// TODO this may need to accommodate non-brew installable dependencies
	HostDependencies() []string
}

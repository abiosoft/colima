package podman

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

// Name is container runtime name.
const Name = "podman"

var _ environment.Container = (*podmanRuntime)(nil)

func init() {
	environment.RegisterContainer(Name, newRuntime)
}

type podmanRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

// NewContainer creates a new podman runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &podmanRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

// Name is the name of the container runtime. e.g. docker, containerd
func (p podmanRuntime) Name() string {
	return Name
}

// Provision provisions/installs the container runtime.
// Should be idempotent.
func (p podmanRuntime) Provision() error {
	a := p.Init()
	a.Stage("Provisioning")
	// check installation
	if !p.isInstalled() {
		a.Stage("provisioning in VM")
		a.Add(p.setupInVM)
	}
	return a.Exec()
}

// Start starts the container runtime.
func (p podmanRuntime) Start() error {
	panic("not implemented") // TODO: Implement
}

// Stop stops the container runtime.
func (p podmanRuntime) Stop() error {
	panic("not implemented") // TODO: Implement
}

// Teardown tears down/uninstall the container runtime.
func (p podmanRuntime) Teardown() error {
	panic("not implemented") // TODO: Implement
}

// Version returns the container runtime version.
func (p podmanRuntime) Version() string {
	panic("not implemented") // TODO: Implement
}

// Running returns if the container runtime is currently running.
func (p podmanRuntime) Running() bool {
	panic("not implemented") // TODO: Implement
}

// Dependencies are dependencies that must exist on the host.
// TODO this may need to accommodate non-brew installable dependencies
func (p podmanRuntime) Dependencies() []string {
	panic("not implemented") // TODO: Implement
}

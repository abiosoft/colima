package containerd

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/runtime"
	"github.com/abiosoft/colima/runtime/container"
)

// Name is container runtime name
const Name = "containerd"

// This is written with assumption that Lima is the VM,
// which provides nerdctl/containerd support out of the box.
// There may be need to make this flexible for non-Lima VMs.

func newRuntime(host runtime.HostActions, guest runtime.GuestActions) container.Container {
	return &containerdRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New("containerd"),
	}
}

func init() {
	container.Register(Name, newRuntime)
}

var _ container.Container = (*containerdRuntime)(nil)

type containerdRuntime struct {
	host  runtime.HostActions
	guest runtime.GuestActions
	cli.CommandChain
}

func (c containerdRuntime) Name() string {
	return Name
}

func (c containerdRuntime) Provision() error {
	// already provisioned as part of Lima
	return nil
}

func (c containerdRuntime) Start() error {
	r := c.Init()
	r.Stage("starting")
	r.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "start")
	})
	return r.Exec()
}

func (c containerdRuntime) Stop() error {
	r := c.Init()
	r.Stage("stopping")
	r.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "stop")
	})
	return r.Exec()
}

func (c containerdRuntime) Teardown() error {
	// teardown not needed, will be part of VM teardown
	return nil
}

func (c containerdRuntime) Dependencies() []string {
	// no dependencies
	return nil
}

func (c containerdRuntime) Version() string {
	version, _ := c.guest.RunOutput("sudo", "nerdctl", "version", "--format", `client: {{.Client.Version}}{{printf "\n"}}server: {{(index .Server.Components 0).Version}}`)
	return version
}

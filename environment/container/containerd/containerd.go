package containerd

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"strconv"
)

// Name is container runtime name
const Name = "containerd"

// This is written with assumption that Lima is the VM,
// which provides nerdctl/containerd support out of the box.
// There may be need to make this flexible for non-Lima VMs.

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &containerdRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func init() {
	environment.RegisterContainer(Name, newRuntime)
}

var _ environment.Container = (*containerdRuntime)(nil)

type containerdRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

func (c containerdRuntime) Name() string {
	return Name
}

func (c containerdRuntime) Provision() error {
	// already provisioned as part of Lima
	return nil
}

const multiArchKey = "containerd_multi_arch"

func (c containerdRuntime) Start() error {
	a := c.Init()
	a.Stage("starting")
	a.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "start")
	})

	a.Add(func() error {
		enabled, _ := strconv.ParseBool(c.guest.Get(multiArchKey))
		if !enabled {
			err := c.guest.Run("sudo", "nerdctl", "run", "--privileged", "--rm", "tonistiigi/binfmt", "--install", "all")
			if err == nil {
				_ = c.guest.Set(multiArchKey, "true")
				_ = c.guest.Run("sudo", "nerdctl", "rmi", "tonistiigi/binfmt")
			}
		}
		return nil
	})

	return a.Exec()
}

func (c containerdRuntime) Running() bool {
	return c.guest.RunQuiet("service", "containerd", "status") == nil
}

func (c containerdRuntime) Stop() error {
	a := c.Init()
	a.Stage("stopping")
	a.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "stop")
	})
	return a.Exec()
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

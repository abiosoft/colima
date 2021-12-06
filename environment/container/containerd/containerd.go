package containerd

import (
	"fmt"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
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

func (c containerdRuntime) Start() error {
	a := c.Init()
	log := c.Logger()

	a.Stage("starting")
	a.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "start")
	})

	// service startup takes few seconds, retry at most 10 times before giving up.
	a.Retry("waiting for startup to complete", time.Second*5, 10, func() error {
		return c.guest.RunQuiet("sudo", "nerdctl", "info")
	})

	a.Add(func() error {
		if err := c.guest.Run("sudo", "nerdctl", "load", "-i", environment.BinfmtTarFile); err != nil {
			log.Warnln(fmt.Errorf("could not enable multi-arch images: %w", err))
		}
		if err := c.guest.Run("sudo", "nerdctl", "run", "--privileged", "--rm", "colima-binfmt", "--install", "all"); err != nil {
			log.Warnln(fmt.Errorf("could not enable multi-arch images: %w", err))
		}
		if err := c.guest.Run("sudo", "nerdctl", "rmi", "--force", "colima-binfmt"); err != nil {
			log.Warnln(fmt.Errorf("could not clear image cache for multi-arch images: %w", err))
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

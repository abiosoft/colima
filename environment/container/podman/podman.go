package podman

import (
	"fmt"
	"os/user"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

const (
	// Name is container runtime name.
	Name          = "podman"
	AARCH64Image  = "https://github.com/hown3d/alpine-lima/releases/download/podman-colima/alpine-lima-clmp-3.15.0-aarch64.iso"
	AARCH64Digest = "sha512:29c740cbcea9acb1779e30a4f8540a8dafddc87d63d65fcbac1f0d7e2011de2abaeb4a33162243e94ed3fd14217a2a146e7da6e35a456b13d74e4a73761cfe50"
	X86_64Image   = "https://github.com/hown3d/alpine-lima/releases/download/podman-colima/alpine-lima-clmp-3.15.0-x86_64.iso"
	X86_64Digest  = "sha512:8e0a975c2da5477c66a49940900d806caf9abc4502cac26845486cba084c6141818c001b10975f2eb524916721896f7904fbd2d9738af6a5900be9e98a1f0289"
)

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
	a.Stage("provisioning")

	// podman context
	a.Add(p.setupConnection)

	return a.Exec()
}

// Start starts the container runtime.
func (p podmanRuntime) Start() error {
	a := p.Init()
	a.Stage("starting")
	if !p.Running() {
		// rootfull
		a.Add(func() error {
			err := p.guest.RunBackground("sudo", "service", "podman", "start")
			if err != nil {
				return fmt.Errorf("Error running rootfull podman: %w", err)
			}
			p.guest.Run("sleep", "5")
			// set permissions of rootfull podman socket to user for accessing via ssh
			// docker has the docker group which podman doesnt have :/
			user, err := user.Current()
			if err != nil {
				return fmt.Errorf("Couldn't read current user: %w", err)
			}
			return p.guest.Run("sudo", "chown", user.Name+":"+user.Name, "-R", "/run/podman/")

		})
	}
	return a.Exec()
}

// Stop stops the container runtime.
func (p podmanRuntime) Stop() error {
	a := p.Init()
	a.Stage("stopping")
	a.Add(func() error {
		if !p.Running() {
			return nil
		}
		return p.guest.Run("sudo", "service", "podman", "stop")
	})

	return a.Exec()
}

// Teardown tears down/uninstall the container runtime.
func (p podmanRuntime) Teardown() error {
	a := p.Init()
	a.Stage("deleting context")
	// no need to uninstall as the VM teardown will remove all components
	// only host configurations should be removed
	a.Add(func() error {
		return p.host.Run("podman", "system", "connection", "rm", config.Profile().ID)
	})
	return a.Exec()
}

// Version returns the container runtime version.
func (p podmanRuntime) Version() string {
	hostVersion, _ := p.host.RunOutput("podman", "--version")
	vmVersion, _ := p.guest.RunOutput("podman", "info", "--format", "{{.Version.Version}}")
	return fmt.Sprintf("client version: %v\nserver version: %v", hostVersion, vmVersion)
}

// Running returns if the container runtime is currently running.
func (p podmanRuntime) Running() bool {
	return p.guest.RunQuiet("service", "podman", "status") == nil
}

// Dependencies are dependencies that must exist on the host.
// TODO this may need to accommodate non-brew installable dependencies
func (p podmanRuntime) Dependencies() []string {
	return []string{"podman"}
}

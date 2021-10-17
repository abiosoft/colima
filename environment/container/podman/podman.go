package podman

import (
	"fmt"
	"strconv"
	"strings"

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
	port := p.guest.Get(environment.SSHPortKey)
	added, err := p.checkIfPodmanRemoteConnectionIsValid(port)
	if err != nil {
		return fmt.Errorf("Can't check remote podman connection: %v", err)
	}
	if !added {
		a.Add(func() error {
			port, err := strconv.Atoi(p.guest.Get(environment.SSHPortKey))
			if err != nil {
				return fmt.Errorf("invalid SSH port: %w", err)
			}
			if port == 0 {
				return fmt.Errorf("SSH port config missing in VM")
			}
			return p.createPodmanConnectionOnHost(port, "colima")
		})
	}

	return a.Exec()
}

// Start starts the container runtime.
func (p podmanRuntime) Start() error {
	a := p.Init()
	a.Stage("starting")
	running, err := p.checkIfPodmanSocketIsRunning()
	if err != nil {
		return err
	}
	if !running {
		a.Add(func() error {
			return p.guest.RunBackground("podman", "system", "service", "-t=0")
		})
	}
	return a.Exec()
}

// Stop stops the container runtime.
func (p podmanRuntime) Stop() error {
	a := p.Init()
	a.Stage("stopping")
	a.Add(func() error {
		output, err := p.guest.RunOutput("pidof", "podman")
		if err != nil {
			return fmt.Errorf("Can't get pids of podman system socket process in VM: %v", err)
		}

		pids := strings.Split(output, " ")
		args := append([]string{"kill", "-9"}, pids...)
		return p.guest.Run(args...)
	})
	return a.Exec()
}

// Teardown tears down/uninstall the container runtime.
func (p podmanRuntime) Teardown() error {
	a := p.Init()
	a.Stage("deleting")
	// no need to uninstall as the VM teardown will remove all components
	// only host configurations should be removed
	a.Add(func() error {
		return p.host.Run("podman", "system", "connection", "rm", "colima")
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
	panic("not implemented") // TODO: Implement
}

// Dependencies are dependencies that must exist on the host.
// TODO this may need to accommodate non-brew installable dependencies
func (p podmanRuntime) Dependencies() []string {
	return []string{"podman"}
}

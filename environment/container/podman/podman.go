package podman

import (
	"fmt"
	"os/user"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
)

const (
	// Name is container runtime name.
	Name = "podman"
	// podman has a compatabil api to docker, so port-forwarding podman.sock as docker.sock works for docker native apps
	dockerSocketPath = "/var/run/docker.sock"
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
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("Couldn't read current user: %v", err)
	}

	a := p.Init()
	a.Stage("Provisioning")
	// check installation
	if !p.isInstalled() {
		a.Stage("provisioning in VM")
		a.Add(p.setupInVM)
	}
	rootfullSocketPath := p.getPodmanSocket(user, true)
	rootlessSocketPath := p.getPodmanSocket(user, false)
	// check symlink
	if !p.isSymlinkCreated(dockerSocketPath) {
		a.Stage("setting up socket")
		a.Add(func() error {
			return p.setupSocketSymlink(dockerSocketPath)
		})
	}
	sshPort, err := p.getSSHPortFromLimactl()
	if err != nil {
		return err
	}
	validRootless, err := p.checkIfPodmanRemoteConnectionIsValid(sshPort, "colima")
	if err != nil {
		return err
	}

	validRootfull, err := p.checkIfPodmanRemoteConnectionIsValid(sshPort, "colima-root")
	if !validRootfull || !validRootless {
		a.Add(func() error {
			return p.createPodmanConnectionOnHost(user, sshPort, "colima", rootfullSocketPath, rootlessSocketPath)
		})
	}
	a.Stage("forwarding podman socket")
	// socket file
	a.Add(func() error {
		return docker.CreateSocketForwardingScript(user.Name, sshPort, rootfullSocketPath, socketSymlink())
	})
	a.Add(func() error {
		return p.host.RunBackground("sh", "-c", config.Dir()+"/socket.sh")
	})

	return a.Exec()
}

// Start starts the container runtime.
func (p podmanRuntime) Start() error {
	a := p.Init()
	a.Stage("starting")
	running, err := p.checkIfPodmanIsRunning()
	if err != nil {
		return err
	}
	if !running {
		// rootless
		a.Add(func() error {
			return p.guest.RunBackground("podman", "system", "service", "-t=0")
		})
		// rootfull
		a.Add(func() error {
			err := p.guest.RunBackground("sudo", "podman", "system", "service", "-t=0")
			if err != nil {
				return fmt.Errorf("Error running rootfull podman: %v", err)
			}
			p.guest.Run("sleep", "5")
			// set permissions of rootfull podman socket to user for accessing via ssh
			// docker has the docker group which podman doesnt have :/
			user, err := user.Current()
			if err != nil {
				return fmt.Errorf("Couldn't read current user: %v", err)
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
		return p.host.Run("podman", "system", "connection", "rm", "colima-root")
	})
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
	running, _ := p.checkIfPodmanIsRunning()
	return running
}

// Dependencies are dependencies that must exist on the host.
// TODO this may need to accommodate non-brew installable dependencies
func (p podmanRuntime) Dependencies() []string {
	return []string{"podman"}
}

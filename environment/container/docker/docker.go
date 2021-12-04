package docker

import (
	"fmt"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

const (
	// Name is container runtime name.
	Name          = "docker"
	AARCH64Image  = "https://github.com/hown3d/alpine-lima/releases/download/podman-colima/alpine-lima-clmd-3.15.0-aarch64.iso"
	AARCH64Digest = "sha512:1d4d8bd5d24affc59c65a49e5d3a5a6075f8ca68990f48e049f0710a2dc6c0d8ef8dc0a276eced934733ccdd74a874b723f506e90b3ab7d9c75dea3562f89893"
	X86_64Image   = "https://github.com/hown3d/alpine-lima/releases/download/podman-colima/alpine-lima-clmd-3.15.0-x86_64.iso"
	X86_64Digest  = "sha512:c93e709a37349dc1c65d20ed485e78a372a6afe88c2d7180ee31a199b0fd035c1393e479b8adbd846cff35f5e12f93ef9e6c4b3411ffc27278c5c83148235575"
)

var _ environment.Container = (*dockerRuntime)(nil)

func init() {
	environment.RegisterContainer(Name, newRuntime)
}

type dockerRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

// NewContainer creates a new docker runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &dockerRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func (d dockerRuntime) Name() string {
	return Name
}

func (d dockerRuntime) isUserPermissionFixed() bool {
	err := d.guest.RunQuiet("sh", "-c", `getent group docker | grep "\b${USER}\b"`)
	return err == nil
}

func (d dockerRuntime) Provision() error {
	a := d.Init()
	a.Stage("provisioning")

	// check user permission
	if !d.isUserPermissionFixed() {
		a.Add(d.fixUserPermission)

		a.Stage("restarting VM to complete setup")
		a.Add(d.guest.Restart)
	}

	if !d.isDaemonFileCreated() {
		a.Add(d.createDaemonFile)
	}

	// daemon.json
	a.Add(d.setupDaemonFile)

	// docker context
	a.Add(d.setupContext)
	a.Add(d.useContext)

	return a.Exec()
}

func (d dockerRuntime) Start() error {
	a := d.Init()
	log := d.Logger()
	a.Stage("starting")

	a.Add(func() error {
		defer time.Sleep(time.Second * 5) // service startup takes few seconds
		return d.guest.Run("sudo", "service", "docker", "start")
	})

	a.Add(func() error {
		if err := d.guest.Run("docker", "load", "-i", environment.BinfmtTarFile); err != nil {
			log.Warnln(fmt.Errorf("could not enable multi-arch images: %w", err))
		}
		if err := d.guest.Run("docker", "run", "--privileged", "--rm", "colima-binfmt", "--install", "all"); err != nil {
			log.Warnln(fmt.Errorf("could not enable multi-arch images: %w", err))
		}
		if err := d.guest.Run("docker", "rmi", "--force", "colima-binfmt"); err != nil {
			log.Warnln(fmt.Errorf("could not clear image cache for multi-arch images: %w", err))
		}
		return nil
	})

	return a.Exec()
}

func (d dockerRuntime) Running() bool {
	return d.guest.RunQuiet("service", "docker", "status") == nil
}

func (d dockerRuntime) Stop() error {
	a := d.Init()
	a.Stage("stopping")

	a.Add(func() error {
		if !d.Running() {
			return nil
		}
		return d.guest.Run("sudo", "service", "docker", "stop")
	})

	return a.Exec()
}

func (d dockerRuntime) Teardown() error {
	a := d.Init()
	a.Stage("deleting")

	// clear docker context settings
	a.Add(d.teardownContext)

	return a.Exec()
}

func (d dockerRuntime) Dependencies() []string {
	return []string{"docker"}
}

func (d dockerRuntime) Version() string {
	version, _ := d.host.RunOutput("docker", "version", "--format", `client: v{{.Client.Version}}{{printf "\n"}}server: v{{.Server.Version}}`)
	return version
}

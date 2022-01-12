package docker

import (
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

// Name is container runtime name.
const Name = "docker"

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

	a.Stage("starting")

	a.Add(func() error {
		return d.guest.Run("sudo", "service", "docker", "start")
	})

	// service startup takes few seconds, retry at most 5 times before giving up.
	a.Retry("waiting for startup to complete", time.Second*5, 10, func() error {
		return d.guest.RunQuiet("sudo", "docker", "info")
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

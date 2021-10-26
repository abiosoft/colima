package docker

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"os"
	"strconv"
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
	launchd launchAgent
}

// NewContainer creates a new docker runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	launchdPkg := "com.abiosoft." + config.Profile()

	return &dockerRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
		launchd:      launchAgent(launchdPkg),
	}
}

func (d dockerRuntime) Name() string {
	return Name
}

func (d dockerRuntime) isInstalled() bool {
	err := d.guest.RunQuiet("command", "-v", "docker")
	return err == nil
}

func (d dockerRuntime) isUserPermissionFixed() bool {
	err := d.guest.RunQuiet("sh", "-c", `getent group docker | grep "\b${USER}\b"`)
	return err == nil
}

func (d dockerRuntime) Provision() error {
	a := d.Init()
	a.Stage("provisioning")

	// check installation
	if !d.isInstalled() {
		a.Stage("provisioning in VM")
		a.Add(d.setupInVM)
	}

	// check user permission
	if !d.isUserPermissionFixed() {
		a.Add(d.fixUserPermission)

		a.Stage("restarting VM to complete setup")
		a.Add(d.guest.Restart)
	}

	// socket file/launchd
	a.Add(func() error {
		user, err := d.guest.User()
		if err != nil {
			return err
		}
		port, err := strconv.Atoi(d.guest.Get(environment.SSHPortKey))
		if err != nil {
			return fmt.Errorf("invalid SSH port: %w", err)
		}
		if port == 0 {
			return fmt.Errorf("SSH port config missing in VM")
		}
		return createSocketForwardingScript(user, port)
	})
	a.Add(func() error { return createLaunchdScript(d.launchd) })

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
	a.Add(func() error {
		return d.host.RunQuiet("launchctl", "load", d.launchd.File())
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
	a.Add(func() error {
		return d.host.RunQuiet("launchctl", "unload", d.launchd.File())
	})

	return a.Exec()
}

func (d dockerRuntime) Teardown() error {
	a := d.Init()
	a.Stage("deleting")

	// no need to uninstall as the VM teardown will remove all components
	// only host configurations should be removed
	if stat, err := os.Stat(d.launchd.File()); err == nil && !stat.IsDir() {
		a.Add(func() error {
			return d.host.RunQuiet("launchctl", "unload", d.launchd.File())
		})
		a.Add(func() error {
			return d.host.RunQuiet("rm", "-rf", d.launchd.File())
		})
	}

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

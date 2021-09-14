package docker

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cli/runner"
	"github.com/abiosoft/colima/clog"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/cruntime"
	"os"
	"path/filepath"
)

var _ cruntime.Runtime = (*Docker)(nil)

const (
	dockerSocket = "/var/run/docker.sock"
)

func dockerSocketSymlink() string {
	return filepath.Join(config.Dir(), "docker.sock")
}

type Docker struct {
	c   cli.Controller
	log *clog.Logger
}

func (d Docker) Name() string {
	return "docker"
}

func (d Docker) isInstalled() bool {
	err := d.c.Guest().Run("command", "-v", "docker")
	return err == nil
}

func (d Docker) isUserPermissionFixed() bool {
	err := d.c.Guest().Run("sh", "-c", `getent group docker | grep "\b${USER}\b"`)
	return err == nil
}

func (d Docker) Provision() error {
	r := d.newRunner()
	r.Stage("provisioning")

	// check installation
	if !d.isInstalled() {
		r.Stage("setting up socket")
		r.Add(d.setupSocketSymlink)

		r.Stage("provisioning in VM")
		r.Add(d.setupInVM)
	}

	// check user permission
	if !d.isUserPermissionFixed() {
		r.Add(d.fixUserPermission)

		r.Stage("restarting VM to complete setup")
		r.Add(d.c.Guest().Stop)
		r.Add(d.c.Guest().Start)
	}

	// socket file/launchd
	r.Add(createSocketForwardingScript)
	r.Add(createLaunchdScript)

	return r.Run()
}

func (d Docker) Start() error {
	r := d.newRunner()
	r.Stage("starting")

	r.Add(func() error {
		return d.c.Guest().Run("sudo", "service", "docker", "start")
	})
	r.Add(func() error {
		return d.c.Host().Run("launchctl", "load", launchdFile())
	})

	return r.Run()
}

func (d Docker) Stop() error {
	r := d.newRunner()
	r.Stage("stopping")

	r.Add(func() error {
		return d.c.Guest().Run("service", "docker", "status")
	})
	r.Add(func() error {
		return d.c.Host().Run("launchctl", "unload", launchdFile())
	})

	return r.Run()
}

func (d Docker) Teardown() error {
	r := d.newRunner()
	r.Stage("teardown")

	if stat, err := os.Stat(launchdFile()); err == nil && !stat.IsDir() {
		r.Add(func() error {
			return d.c.Host().Run("launchctl", "unload", launchdFile())
		})
	}

	return r.Run()
}

func (d Docker) HostDependencies() []string {
	return []string{"docker"}
}

func (d Docker) newRunner() *runner.Runner { return runner.New(d.Name()) }

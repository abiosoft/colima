package docker

import (
	"context"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
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

func (d dockerRuntime) Provision(ctx context.Context) error {
	a := d.Init()
	log := d.Logger()
	a.Stage("provisioning")

	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	// daemon.json
	a.Add(func() error {
		// not a fatal error
		if err := d.createDaemonFile(conf.Docker); err != nil {
			log.Warnln(err)
		}
		return nil
	})

	// docker context
	a.Add(d.setupContext)
	a.Add(d.useContext)

	return a.Exec()
}

func (d dockerRuntime) Start(ctx context.Context) error {
	a := d.Init()

	a.Stage("starting")

	a.Add(func() error {
		return d.guest.Run("sudo", "service", "docker", "start")
	})

	// service startup takes few seconds, retry at most 5 times before giving up.
	a.Retry("", time.Second*5, 10, func(int) error {
		return d.guest.RunQuiet("sudo", "docker", "info")
	})

	return a.Exec()
}

func (d dockerRuntime) Running() bool {
	return d.guest.RunQuiet("service", "docker", "status") == nil
}

func (d dockerRuntime) Stop(ctx context.Context) error {
	a := d.Init()
	a.Stage("stopping")

	a.Add(func() error {
		if !d.Running() {
			return nil
		}
		return d.guest.Run("sudo", "service", "docker", "stop")
	})

	// clear docker context settings
	// since the container runtime can be changed on startup,
	// it is better to not leave unnecessary traces behind
	a.Add(d.teardownContext)

	return a.Exec()
}

func (d dockerRuntime) Teardown(ctx context.Context) error {
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

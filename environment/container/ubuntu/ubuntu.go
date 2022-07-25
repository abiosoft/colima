package ubuntu

import (
	"context"
	"fmt"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
)

// Name is container runtime name
const Name = "ubuntu"
const containerdNamespace = "colima"
const containerName = "ubuntu-layer"
const imageArchive = "/usr/share/colima/ubuntu-layer.tar.gz"
const imageName = "ubuntu-layer"

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &ubuntuRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func nerdctl(args ...string) []string {
	return append([]string{"nerdctl", "--namespace", containerdNamespace}, args...)
}

func init() {
	environment.RegisterContainer(Name, newRuntime, true)
}

type ubuntuRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

func (u ubuntuRuntime) Name() string {
	return Name
}

func (u ubuntuRuntime) ensureContainerd(ctx context.Context) error {
	nerd, err := environment.NewContainer(containerd.Name, u.host, u.guest)
	if err != nil {
		return fmt.Errorf("%s required for ubuntu layer: %w", containerd.Name, err)
	}
	if nerd.Running(ctx) {
		return nil
	}

	ctx = context.WithValue(ctx, cli.CtxKeyQuiet, true)
	if err := nerd.Provision(ctx); err != nil {
		return err
	}

	return nerd.Start(ctx)
}

func (u ubuntuRuntime) Provision(ctx context.Context) error {
	a := u.Init(ctx)
	if err := u.ensureContainerd(ctx); err != nil {
		return err
	}

	if !u.imageCreated() {
		a.Stage("creating image")
		a.Add(u.createImage)
	}

	conf, _ := ctx.Value(config.CtxKey()).(config.Config)
	if !u.containerCreated() {
		a.Stage("creating container")
		a.Add(func() error {
			return u.createContainer(conf)
		})
	}

	return a.Exec()
}

func (u ubuntuRuntime) Start(ctx context.Context) error {
	a := u.Init(ctx)

	a.Add(func() error {
		return u.guest.Run(nerdctl("start", containerName)...)
	})
	a.Add(u.syncHostname)

	return a.Exec()
}

func (u ubuntuRuntime) Stop(context.Context) error {
	return u.guest.Run(nerdctl("stop", containerName)...)
}

func (u ubuntuRuntime) Teardown(context.Context) error {
	return u.guest.Run(nerdctl("rm", "-f", containerName)...)
}

func (u ubuntuRuntime) Version(ctx context.Context) string {
	args := nerdctl("exec", "--", "sh -c '. /etc/os-release && echo $PRETTY_NAME'")
	out, _ := u.guest.RunOutput(args...)
	return out
}

func (u ubuntuRuntime) Running(ctx context.Context) bool {
	args := nerdctl("exec", containerName, "uname")
	return u.guest.RunQuiet(args...) == nil
}

func (u ubuntuRuntime) Dependencies() []string {
	return nil
}

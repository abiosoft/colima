package containerd

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

// Name is container runtime name
const Name = "containerd"

var configDir = func() string { return config.CurrentProfile().ConfigDir() }

// HostSocketFile returns the path to the containerd socket on host.
func HostSocketFile() string { return filepath.Join(configDir(), "containerd.sock") }

// This is written with assumption that Lima is the VM,
// which provides nerdctl/containerd support out of the box.
// There may be need to make this flexible for non-Lima VMs.

//go:embed buildkitd.toml
var buildKitConf []byte

const buildKitConfFile = "/etc/buildkit/buildkitd.toml"

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &containerdRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func init() {
	environment.RegisterContainer(Name, newRuntime, false)
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

func (c containerdRuntime) Provision(context.Context) error {
	return c.guest.Write(buildKitConfFile, buildKitConf)
}

func (c containerdRuntime) Start(ctx context.Context) error {
	a := c.Init(ctx)

	a.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "restart")
	})

	// service startup takes few seconds, retry at most 10 times before giving up.
	a.Retry("", time.Second*5, 10, func(int) error {
		return c.guest.RunQuiet("sudo", "nerdctl", "info")
	})

	a.Add(func() error {
		return c.guest.Run("sudo", "service", "buildkit", "start")
	})

	return a.Exec()
}

func (c containerdRuntime) Running(ctx context.Context) bool {
	return c.guest.RunQuiet("service", "containerd", "status") == nil
}

func (c containerdRuntime) Stop(ctx context.Context) error {
	a := c.Init(ctx)
	a.Add(func() error {
		return c.guest.Run("sudo", "service", "containerd", "stop")
	})
	return a.Exec()
}

func (c containerdRuntime) Teardown(context.Context) error {
	// teardown not needed, will be part of VM teardown
	return nil
}

func (c containerdRuntime) Dependencies() []string {
	// no dependencies
	return nil
}

func (c containerdRuntime) Version(ctx context.Context) string {
	version, _ := c.guest.RunOutput("sudo", "nerdctl", "version", "--format", `client: {{.Client.Version}}{{printf "\n"}}server: {{(index .Server.Components 0).Version}}`)
	return version
}

func (c *containerdRuntime) Update(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("update not supported for the %s runtime", Name)
}

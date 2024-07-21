package incus

import (
	"context"
	_ "embed"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &containerdRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

var configDir = func() string { return config.CurrentProfile().ConfigDir() }

// HostSocketFile returns the path to the containerd socket on host.
func HostSocketFile() string { return filepath.Join(configDir(), "incus.sock") }

const Name = "incus"

func init() {
	environment.RegisterContainer(Name, newRuntime, false)
}

var _ environment.Container = (*containerdRuntime)(nil)

type containerdRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain

	running bool
}

// Dependencies implements environment.Container.
func (c *containerdRuntime) Dependencies() []string { return nil }

// Provision implements environment.Container.
func (c *containerdRuntime) Provision(ctx context.Context) error { return nil }

// Running implements environment.Container.
func (c *containerdRuntime) Running(ctx context.Context) bool { return c.running }

// Start implements environment.Container.
func (c *containerdRuntime) Start(ctx context.Context) error { c.running = true; return nil }

// Stop implements environment.Container.
func (c *containerdRuntime) Stop(ctx context.Context) error { c.running = false; return nil }

// Teardown implements environment.Container.
func (c *containerdRuntime) Teardown(ctx context.Context) error { return nil }

// Version implements environment.Container.
func (c *containerdRuntime) Version(ctx context.Context) string { return "v1" }

func (c containerdRuntime) Name() string {
	return Name
}

//go:embed config.yaml
var configYaml string

// cat incus.yaml | incus admin init --preseed
// detect with storage list
// add docker remote

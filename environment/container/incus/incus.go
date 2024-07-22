package incus

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
)

const incusBridgeInterface = "incusbr0"

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &incusRuntime{
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

var _ environment.Container = (*incusRuntime)(nil)

type incusRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

// Dependencies implements environment.Container.
func (c *incusRuntime) Dependencies() []string {
	return []string{"incus"}
}

// Provision implements environment.Container.
func (c *incusRuntime) Provision(ctx context.Context) error {
	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	if err := c.guest.RunQuiet("ip", "addr", "show", incusBridgeInterface); err == nil {
		// already provisioned
		return nil
	}

	var value struct {
		Disk      int
		Interface string
	}
	value.Disk = conf.Disk - 5 // use all disk except 5GiB
	value.Interface = incusBridgeInterface

	buf, err := util.ParseTemplate(configYaml, value)
	if err != nil {
		return fmt.Errorf("error parsing incus config template: %w", err)
	}

	stdin := bytes.NewReader(buf)
	if err := c.guest.RunWith(stdin, nil, "incus", "admin", "init", "--preseed"); err != nil {
		return fmt.Errorf("error setting up incus: %w", err)
	}

	return nil
}

// Running implements environment.Container.
func (c *incusRuntime) Running(ctx context.Context) bool {
	return c.guest.RunQuiet("service", "incus", "status") == nil
}

// Start implements environment.Container.
func (c *incusRuntime) Start(ctx context.Context) error {
	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	a := c.Init(ctx)

	a.Add(func() error {
		return c.guest.RunQuiet("sudo", "service", "incus", "start")
	})

	a.Add(func() error {
		return c.setRemote(conf.AutoActivate())
	})

	return a.Exec()
}

// Stop implements environment.Container.
func (c *incusRuntime) Stop(ctx context.Context) error {
	a := c.Init(ctx)

	a.Add(func() error {
		return c.guest.RunQuiet("sudo", "service", "incus", "stop")
	})

	a.Add(c.unsetRemote)

	return a.Exec()
}

// Teardown implements environment.Container.
func (c *incusRuntime) Teardown(ctx context.Context) error { return nil }

// Version implements environment.Container.
func (c *incusRuntime) Version(ctx context.Context) string {
	version, _ := c.guest.RunOutput("incus", "version")
	return version
}

func (c incusRuntime) Name() string {
	return Name
}

func (c incusRuntime) setRemote(activate bool) error {
	// add remote
	if !c.hasRemote() {
		if err := c.host.RunQuiet("incus", "remote", "add", config.CurrentProfile().ID, "unix://"+HostSocketFile()); err != nil {
			return err
		}
	}

	// if activate, set default to new remote
	if activate {
		return c.host.RunQuiet("incus", "remote", "switch", config.CurrentProfile().ID)
	}

	return nil
}

func (c incusRuntime) unsetRemote() error {
	// if default remote, set default to local
	if c.isDefaultRemote() {
		if err := c.host.RunQuiet("incus", "remote", "switch", "local"); err != nil {
			return err
		}
	}

	// if has remote, remove remote
	if c.hasRemote() {
		return c.host.RunQuiet("incus", "remote", "remove", config.CurrentProfile().ID)
	}

	return nil
}

func (c incusRuntime) hasRemote() bool {
	return c.host.RunQuiet("sh", "-c", "incus remote list | grep "+HostSocketFile()) == nil
}

func (c incusRuntime) isDefaultRemote() bool {
	remote, _ := c.host.RunOutput("incus", "remote", "get-default")
	return remote == config.CurrentProfile().ID
}

//go:embed config.yaml
var configYaml string

// cat incus.yaml | incus admin init --preseed
// detect with netword bridge 'ip addr show incusbr0'
// add docker remote
// disable kubernetes for incus

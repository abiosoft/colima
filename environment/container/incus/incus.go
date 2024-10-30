package incus

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/debutil"
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
	value.Disk = conf.Disk - 5 // use all disk except 5GiB. TODO: revisit.
	value.Interface = incusBridgeInterface

	buf, err := util.ParseTemplate(configYaml, value)
	if err != nil {
		return fmt.Errorf("error parsing incus config template: %w", err)
	}

	stdin := bytes.NewReader(buf)
	if err := c.guest.RunWith(stdin, nil, "sudo", "incus", "admin", "init", "--preseed"); err != nil {
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
		// attempt to set remote
		if err := c.setRemote(conf.AutoActivate()); err == nil {
			return nil
		}

		// workaround missing user in incus-admin by restarting
		ctx := context.WithValue(ctx, cli.CtxKeyQuiet, true)
		if err := c.guest.Restart(ctx); err != nil {
			return err
		}

		// attempt once again to set remote
		return c.setRemote(conf.AutoActivate())
	})

	a.Add(func() error {
		if err := c.addDockerRemote(); err != nil {
			return cli.ErrNonFatal(err)
		}
		return nil
	})

	a.Add(func() error {
		if err := c.registerNetworks(); err != nil {
			return cli.ErrNonFatal(err)
		}
		return nil
	})

	return a.Exec()
}

// Stop implements environment.Container.
func (c *incusRuntime) Stop(ctx context.Context) error {
	a := c.Init(ctx)

	a.Add(func() error {
		return c.guest.RunQuiet("sudo", "incus", "admin", "shutdown")
	})

	a.Add(c.unsetRemote)

	return a.Exec()
}

// Teardown implements environment.Container.
func (c *incusRuntime) Teardown(ctx context.Context) error {
	a := c.Init(ctx)

	a.Add(c.unsetRemote)

	return a.Exec()
}

// Version implements environment.Container.
func (c *incusRuntime) Version(ctx context.Context) string {
	version, _ := c.host.RunOutput("incus", "version", config.CurrentProfile().ID+":")
	return version
}

func (c incusRuntime) Name() string {
	return Name
}

func (c incusRuntime) setRemote(activate bool) error {
	name := config.CurrentProfile().ID

	// add remote
	if !c.hasRemote(name) {
		if err := c.host.RunQuiet("incus", "remote", "add", name, "unix://"+HostSocketFile()); err != nil {
			return err
		}
	}

	// if activate, set default to new remote
	if activate {
		return c.host.RunQuiet("incus", "remote", "switch", name)
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
	if c.hasRemote(config.CurrentProfile().ID) {
		return c.host.RunQuiet("incus", "remote", "remove", config.CurrentProfile().ID)
	}

	return nil
}

func (c incusRuntime) hasRemote(name string) bool {
	remotes, err := c.fetchRemotes()
	if err != nil {
		return false
	}

	_, ok := remotes[name]
	return ok
}

func (c incusRuntime) fetchRemotes() (remoteInfo, error) {
	b, err := c.host.RunOutput("incus", "remote", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("error fetching remotes: %w", err)
	}

	var remotes remoteInfo
	if err := json.NewDecoder(strings.NewReader(b)).Decode(&remotes); err != nil {
		return nil, fmt.Errorf("error decoding remotes response: %w", err)
	}

	return remotes, nil
}

func (c incusRuntime) isDefaultRemote() bool {
	remote, _ := c.host.RunOutput("incus", "remote", "get-default")
	return remote == config.CurrentProfile().ID
}

func (c incusRuntime) addDockerRemote() error {
	if c.hasRemote("docker") {
		// already added
		return nil
	}

	return c.host.RunQuiet("incus", "remote", "add", "docker", "https://docker.io", "--protocol=oci")
}

func (c incusRuntime) registerNetworks() error {
	b, err := c.guest.RunOutput("sudo", "incus", "network", "list", "--format", "json")
	if err != nil {
		return fmt.Errorf("error listing networks: %w", err)
	}

	var network networkInfo
	var found bool
	name := limautil.NetInterface
	{ // decode and flatten for easy lookup
		var resp []networkInfo
		if err := json.NewDecoder(strings.NewReader(b)).Decode(&resp); err != nil {
			return fmt.Errorf("error decoding networks into struct: %w", err)
		}
		for _, n := range resp {
			if n.Name == name {
				network = n
				found = true
			}
		}
	}

	// must be an unmanaged physical network
	if !found || network.Managed || network.Type != "physical" {
		return nil
	}

	err = c.guest.RunQuiet("sudo", "incus", "network", "create", name, "--type", "macvlan", "parent="+name)
	if err != nil {
		return fmt.Errorf("error creating managed network '%s': %w", name, err)
	}

	return nil
}

//go:embed config.yaml
var configYaml string

type remoteInfo map[string]struct {
	Addr string `json:"Addr"`
}

type networkInfo struct {
	Name    string `json:"name"`
	Managed bool   `json:"managed"`
	Type    string `json:"type"`
}

func (c *incusRuntime) Update(ctx context.Context) (bool, error) {
	packages := []string{
		"incus",
		"incus-base",
		"incus-client",
		"incus-extra",
		"incus-ui-canonical",
	}

	return debutil.UpdateRuntime(ctx, c.guest, c, packages...)
}

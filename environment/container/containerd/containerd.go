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

// HostSocketFiles returns the path to the socket files on host.
func HostSocketFiles() (files struct {
	Containerd string
	Buildkitd  string
}) {
	files.Containerd = filepath.Join(configDir(), "containerd.sock")
	files.Buildkitd = filepath.Join(configDir(), "buildkitd.sock")

	return files
}

// This is written with assumption that Lima is the VM,
// which provides nerdctl/containerd support out of the box.
// There may be need to make this flexible for non-Lima VMs.

//go:embed config.toml
var containerdConf []byte

//go:embed buildkitd.toml
var buildKitConf []byte

const containerdConfFile = "/etc/containerd/config.toml"
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

func (c containerdRuntime) Provision(ctx context.Context) error {
	a := c.Init(ctx)

	// containerd config
	a.Add(func() error {
		return c.guest.Write(containerdConfFile, containerdConf)
	})

	// buildkitd config
	a.Add(func() error {
		return c.guest.Write(buildKitConfFile, buildKitConf)
	})

	return a.Exec()
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

// DataDirs represents the data disk for the container runtime.
func DataDisk() environment.DataDisk {
	return environment.DataDisk{
		Dirs:   diskDirs,
		FSType: "ext4",
		PreMount: []string{
			"systemctl stop containerd",
			"systemctl stop buildkit",
		},
	}
}

var diskDirs = []environment.DiskDir{
	{Name: "containerd", Path: "/var/lib/containerd"},
	{Name: "buildkit", Path: "/var/lib/buildkit"},
	{Name: "nerdctl", Path: "/var/lib/nerdctl"},
	{Name: "rancher", Path: "/var/lib/rancher"},
	{Name: "cni", Path: "/var/lib/cni"},
}

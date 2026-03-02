package containerd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
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
		profilePath := filepath.Join(configDir(), "containerd", "config.toml")
		centralPath := filepath.Join(userConfigDir(), "containerd", "config.toml")
		return c.provisionConfig(profilePath, centralPath, containerdConfFile, containerdConf)
	})

	// buildkitd config
	a.Add(func() error {
		profilePath := filepath.Join(configDir(), "containerd", "buildkitd.toml")
		centralPath := filepath.Join(userConfigDir(), "buildkit", "buildkitd.toml")
		return c.provisionConfig(profilePath, centralPath, buildKitConfFile, buildKitConf)
	})

	return a.Exec()
}

// userConfigDir returns the user config directory following XDG conventions.
// This is ~/.config on Linux/macOS, used for central config file locations
// that follow the containerd/buildkit rootless conventions.
func userConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config")
}

// provisionConfig writes a config file to the VM. Config files are resolved
// in the following order:
//  1. Per-profile override at ~/.colima/<profile>/containerd/<file>
//  2. Central config at ~/.config/containerd/<file> (or ~/.config/buildkit/<file>)
//  3. Embedded default
//
// On first run, the default config is written to the central location for
// user discovery and editing.
func (c containerdRuntime) provisionConfig(profilePath, centralPath, guestPath string, defaultConf []byte) error {
	// 1. per-profile override takes highest priority
	if data, err := os.ReadFile(profilePath); err == nil {
		return c.guest.Write(guestPath, data)
	}

	// 2. central config
	if data, err := os.ReadFile(centralPath); err == nil {
		return c.guest.Write(guestPath, data)
	}

	// 3. no user config found; write the default to the central location
	// for discoverability and use it
	if err := os.MkdirAll(filepath.Dir(centralPath), 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}
	if err := os.WriteFile(centralPath, defaultConf, 0644); err != nil {
		return fmt.Errorf("error writing default config: %w", err)
	}

	return c.guest.Write(guestPath, defaultConf)
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
			"systemctl stop containerd.service",
			"systemctl stop buildkit.service",
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

package podman

import (
	"context"
	"path/filepath"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/guest/systemctl"
	"github.com/abiosoft/colima/util/debutil"
)

// Name is container runtime name.
const Name = "podman"

var configDir = func() string { return config.CurrentProfile().ConfigDir() }

// HostSocketFile returns the path to the podman socket on host.
func HostSocketFile() string { return filepath.Join(configDir(), "podman.sock") }

func init() {
	environment.RegisterContainer(Name, newRuntime, false)
}

var _ environment.Container = (*podmanRuntime)(nil)

type podmanRuntime struct {
	host      environment.HostActions
	guest     environment.GuestActions
	systemctl systemctl.Systemctl
	cli.CommandChain
}

func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &podmanRuntime{
		host:         host,
		guest:        guest,
		systemctl:    systemctl.New(guest),
		CommandChain: cli.New(Name),
	}
}

func (p podmanRuntime) Name() string { return Name }

func (p podmanRuntime) Provision(ctx context.Context) error {
	a := p.Init(ctx)

	// refresh package index and install podman
	a.Add(func() error {
		return p.guest.Run("sudo", "sh", "-c", "apt-get update -y && apt-get install -y podman")
	})

	// configure podman.socket to be group-accessible (group=podman, mode=0660)
	a.Add(func() error {
		override := []byte("[Socket]\nSocketMode=0660\nSocketGroup=podman\n")
		if err := p.guest.Run("sudo", "mkdir", "-p", "/etc/systemd/system/podman.socket.d"); err != nil {
			return err
		}
		return p.guest.Write("/etc/systemd/system/podman.socket.d/override.conf", override)
	})

	a.Add(func() error {
		return p.guest.Run("sudo", "systemctl", "daemon-reload")
	})

	a.Add(func() error {
		return p.guest.Run("sudo", "systemctl", "enable", "podman.socket")
	})

	return a.Exec()
}

func (p podmanRuntime) Start(ctx context.Context) error {
	a := p.Init(ctx)

	a.Add(func() error {
		return p.systemctl.Start("podman.socket")
	})

	// verify podman is accessible; retry for up to 60 seconds
	a.Retry("", time.Second, 60, func(int) error {
		return p.guest.RunQuiet("sudo", "podman", "info")
	})

	return a.Exec()
}

func (p podmanRuntime) Running(ctx context.Context) bool {
	return p.systemctl.Active("podman.service") || p.systemctl.Active("podman.socket")
}

func (p podmanRuntime) Stop(ctx context.Context, force bool) error {
	a := p.Init(ctx)

	a.Add(func() error {
		return p.systemctl.Stop("podman.service", force)
	})

	a.Add(func() error {
		return p.systemctl.Stop("podman.socket", force)
	})

	return a.Exec()
}

func (p podmanRuntime) Teardown(ctx context.Context) error {
	// teardown not needed, will be part of VM teardown
	return nil
}

func (p podmanRuntime) Version(ctx context.Context) string {
	version, _ := p.guest.RunOutput("sudo", "podman", "version", "--format",
		`client: {{.Client.Version}}{{printf "\n"}}server: {{.Server.Version}}`)
	return version
}

func (p *podmanRuntime) Update(ctx context.Context) (bool, error) {
	return debutil.UpdateRuntime(ctx, p.guest, p, "podman")
}

func (p podmanRuntime) Dependencies() []string {
	// no host-side CLI required; socket is exposed directly
	return nil
}

// DataDisk represents the data disk for the podman runtime.
func DataDisk() environment.DataDisk {
	return environment.DataDisk{
		Dirs: []environment.DiskDir{
			{Name: "podman", Path: "/var/lib/containers"},
		},
		FSType: "ext4",
		PreMount: []string{
			"systemctl stop podman.service || true",
			"systemctl stop podman.socket || true",
		},
	}
}

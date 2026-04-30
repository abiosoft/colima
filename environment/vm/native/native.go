package native

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
)

// New creates a new native VM that runs directly on the host without
// any virtualization. This is only intended for Linux hosts where container
// runtimes can run natively without a VM.
func New(host environment.HostActions) environment.VM {
	return &nativeVM{
		host:         host,
		CommandChain: cli.New("vm"),
	}
}

var _ environment.VM = (*nativeVM)(nil)

type nativeVM struct {
	host environment.HostActions
	cli.CommandChain


	// keep config in case of restart
	conf config.Config
}

func (n nativeVM) Dependencies() []string {
	// No external dependencies needed (no Lima/QEMU)
	return nil
}

func (n *nativeVM) Start(ctx context.Context, conf config.Config) error {
	a := n.Init(ctx)
	log := n.Logger(ctx)

	n.conf = conf

	if n.Running(ctx) {
		log.Println("already running")
		return nil
	}

	a.Stage("starting (native mode)")

	// Save state first so other commands can detect this is a native instance
	a.Add(func() error {
		stateFile := config.CurrentProfile().StateFile()
		stateDir := filepath.Dir(stateFile)
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			return fmt.Errorf("error creating state directory: %w", err)
		}
		if err := configmanager.SaveToFile(conf, stateFile); err != nil {
			return fmt.Errorf("error persisting Colima state: %w", err)
		}
		return nil
	})

	// Verify the container runtime is available on the host
	a.Add(func() error {
		return n.verifyRuntime(conf.Runtime)
	})

	return a.Exec()
}

func (n nativeVM) Stop(ctx context.Context, force bool) error {
	// In native mode, we don't stop a VM. The container runtime
	// stop is handled by the container layer (app.go).
	return nil
}

func (n nativeVM) Teardown(ctx context.Context) error {
	// Clean up the native config file
	configFile := n.configFilePath()
	_ = n.host.RunQuiet("rm", "-f", configFile)
	return nil
}

func (n nativeVM) Running(_ context.Context) bool {
	// First check if this profile has been started by Colima
	// (state file must exist)
	if !n.Created() {
		return false
	}

	// Check if the primary container runtime service is active.
	conf, err := configmanager.LoadInstance()
	if err != nil {
		// State file exists but can't be loaded — check common runtimes
		for _, svc := range []string{"docker.service", "containerd.service", "incus.service"} {
			if n.host.RunQuiet("systemctl", "is-active", svc) == nil {
				return true
			}
		}
		return false
	}

	switch conf.Runtime {
	case docker.Name:
		return n.host.RunQuiet("systemctl", "is-active", "docker.service") == nil
	case containerd.Name:
		return n.host.RunQuiet("systemctl", "is-active", "containerd.service") == nil
	case incus.Name:
		return n.host.RunQuiet("systemctl", "is-active", "incus.service") == nil
	}

	return false
}

func (n *nativeVM) Restart(ctx context.Context) error {
	if n.conf.Empty() {
		return fmt.Errorf("cannot restart, instance not previously started")
	}

	// In native mode, restart means re-running Start which re-verifies
	// the runtime and re-saves state. There is no VM to restart.
	return n.Start(ctx, n.conf)
}

func (n nativeVM) Host() environment.HostActions {
	return n.host
}

func (n nativeVM) Env(s string) (string, error) {
	return n.host.Env(s), nil
}

func (n nativeVM) Created() bool {
	_, err := n.host.Read(config.CurrentProfile().StateFile())
	return err == nil
}

func (n nativeVM) User() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("error getting current user: %w", err)
	}
	return u.Username, nil
}

func (n nativeVM) Arch() environment.Arch {
	return environment.HostArch()
}

// verifyRuntime checks that the specified container runtime is available on the host.
func (n nativeVM) verifyRuntime(runtime string) error {
	if environment.IsNoneRuntime(runtime) {
		return nil
	}

	switch runtime {
	case docker.Name:
		// Check systemctl first, then fallback to docker binary existence
		if n.host.RunQuiet("systemctl", "is-active", "docker.service") != nil {
			// Not active via systemd, check if docker binary exists
			if n.host.RunQuiet("which", "docker") != nil {
				return fmt.Errorf("docker is not available on this host\n" +
					"Install with: curl -fsSL https://get.docker.com | sh")
			}
			return fmt.Errorf("docker is installed but not running\n" +
				"Start with: sudo systemctl start docker")
		}
	case containerd.Name:
		if n.host.RunQuiet("systemctl", "is-active", "containerd.service") != nil {
			return fmt.Errorf("containerd is not available on this host\n" +
				"Install containerd and ensure it is running")
		}
	case incus.Name:
		if n.host.RunQuiet("which", "incus") != nil {
			return fmt.Errorf("incus is not available on this host\n" +
				"Install incus and ensure it is running")
		}
	default:
		return fmt.Errorf("unsupported runtime for native mode: %s", runtime)
	}

	return nil
}

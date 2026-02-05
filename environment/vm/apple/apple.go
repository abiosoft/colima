package apple

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/socktainer"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/abiosoft/colima/environment/vm/apple/appleutil"
	"github.com/abiosoft/colima/util"
)

// Name is the name of the Apple Container backend.
const Name = "apple"

// ContainerCommand is the command for Apple Container CLI.
const ContainerCommand = "container"

func init() {
	// Only register on supported systems (macOS 26+ on Apple Silicon)
	if util.MacOS26OrNewer() && environment.HostArch() == environment.AARCH64 {
		vm.RegisterVM(vm.BackendApple, New)
		vm.RegisterInstanceLister(vm.BackendApple, &appleInstanceLister{})
	}
}

// New creates a new Apple Container VM.
func New(host environment.HostActions) environment.VM {
	return &appleVM{
		host:         host,
		daemon:       daemon.NewManager(host),
		CommandChain: cli.New(Name),
	}
}

var _ environment.VM = (*appleVM)(nil)

type appleVM struct {
	host   environment.HostActions
	daemon daemon.Manager
	cli.CommandChain

	// keep config in case of restart
	conf config.Config
}

// Dependencies returns the dependencies required for Apple Container.
// The container CLI is required and can be installed via 'brew install container'.
// Socktainer is installed automatically when starting the daemon.
func (a appleVM) Dependencies() []string {
	return []string{ContainerCommand}
}

// Host returns the host actions.
func (a appleVM) Host() environment.HostActions {
	return a.host
}

// Start starts the Apple Container system.
func (a *appleVM) Start(ctx context.Context, conf config.Config) error {
	log := a.Logger(ctx)
	chain := a.Init(ctx)

	// Check if another Apple Container instance already exists
	if existingProfile := findExistingAppleInstance(); existingProfile != "" {
		if existingProfile != config.CurrentProfile().ShortName {
			return fmt.Errorf("Apple Container runtime already exists as profile '%s'. Only one instance is supported", existingProfile)
		}
	}

	if a.Running(ctx) {
		log.Println("already running")
		return nil
	}

	chain.Stage("starting Apple Container system")

	// Start the Apple Container system
	chain.Add(func() error {
		return a.host.RunInteractive(ContainerCommand, "system", "start")
	})

	// Wait for system to be running
	chain.Retry("waiting for system", time.Second, 30, func(int) error {
		if !a.Running(ctx) {
			return fmt.Errorf("system not yet running")
		}
		return nil
	})

	// Save config for restart
	chain.Add(func() error {
		a.conf = conf
		return nil
	})

	// Start daemon for socktainer (after system is up)
	chain.Add(func() error {
		return a.startDaemon(ctx, conf)
	})

	return chain.Exec()
}

// Stop stops the Apple Container system.
func (a appleVM) Stop(ctx context.Context, force bool) error {
	log := a.Logger(ctx)
	chain := a.Init(ctx)

	if !a.Running(ctx) && !force {
		log.Println("not running")
		return nil
	}

	chain.Stage("stopping")

	// Stop daemon first
	chain.Add(func() error {
		return a.stopDaemon(ctx, a.conf)
	})

	// Stop the Apple Container system
	chain.Add(func() error {
		return a.host.RunInteractive(ContainerCommand, "system", "stop")
	})

	return chain.Exec()
}

// Restart restarts the Apple Container system.
func (a *appleVM) Restart(ctx context.Context) error {
	if a.conf.Empty() {
		return fmt.Errorf("cannot restart, not previously started")
	}

	if err := a.Stop(ctx, false); err != nil {
		return err
	}

	// Minor delay to prevent possible race condition
	time.Sleep(time.Second * 2)

	return a.Start(ctx, a.conf)
}

// Teardown stops the Apple Container system.
// Unlike Lima VMs, Apple Container system is shared and not deleted.
func (a appleVM) Teardown(ctx context.Context) error {
	chain := a.Init(ctx)

	// Stop daemon first
	chain.Add(func() error {
		return a.stopDaemon(ctx, a.conf)
	})

	// Stop the Apple Container system
	chain.Add(func() error {
		return a.host.RunInteractive(ContainerCommand, "system", "stop")
	})

	return chain.Exec()
}

// startDaemon starts the background daemon for socktainer.
func (a *appleVM) startDaemon(ctx context.Context, conf config.Config) error {
	chain := a.Init(ctx)
	log := chain.Logger()

	// Ensure socktainer is installed before starting daemon
	if err := ensureSocktainer(a.host, log); err != nil {
		return err
	}

	ctxKeySocktainer := daemon.CtxKey(socktainer.Name)

	// Add socktainer to daemon
	chain.Add(func() error {
		ctx = context.WithValue(ctx, ctxKeySocktainer, true)
		deps, _ := a.daemon.Dependency(ctx, conf, socktainer.Name)
		if err := deps.Install(a.host); err != nil {
			return fmt.Errorf("error setting up socktainer dependencies: %w", err)
		}
		return nil
	})

	// Start daemon
	chain.Add(func() error {
		return a.daemon.Start(ctx, conf)
	})

	// Verify daemon is running
	chain.Retry("waiting for daemon", time.Second, 15, func(int) error {
		s, err := a.daemon.Running(ctx, conf)
		if err != nil {
			return err
		}
		if !s.Running {
			return fmt.Errorf("daemon is not running")
		}
		for _, p := range s.Processes {
			if !p.Running {
				return p.Error
			}
		}
		return nil
	})

	if err := chain.Exec(); err != nil {
		log.Warnln(fmt.Errorf("error starting daemon: %w", err))
	}

	return nil
}

// stopDaemon stops the background daemon.
func (a appleVM) stopDaemon(ctx context.Context, conf config.Config) error {
	return a.daemon.Stop(ctx, conf)
}

// Created returns if Colima has previously been set up with Apple Container.
// Checks the config file for apple runtime.
func (a appleVM) Created() bool {
	return appleutil.IsAppleBackend()
}

// Running returns if the Apple Container system is currently running.
func (a appleVM) Running(_ context.Context) bool {
	// Check system status using `container system status`
	return a.host.RunQuiet(ContainerCommand, "system", "status") == nil
}

// Env retrieves an environment variable in the container.
func (a appleVM) Env(s string) (string, error) {
	return "", fmt.Errorf("env is not supported for Apple Container runtime")
}

// User returns the username of the user in the container.
func (a appleVM) User() (string, error) {
	return "", fmt.Errorf("user is not supported for Apple Container runtime")
}

// Arch returns the architecture of the container.
func (a appleVM) Arch() environment.Arch {
	return environment.HostArch()
}

// appleInstanceLister implements vm.InstanceLister for Apple Container.
type appleInstanceLister struct{}

// Instances returns the Apple Container instance (only one can exist).
func (l *appleInstanceLister) Instances(ids ...string) ([]vm.InstanceInfo, error) {
	inst := findAppleInstance()
	if inst == nil {
		return nil, nil
	}

	// Filter by IDs if specified
	if len(ids) > 0 {
		if !slices.Contains(ids, inst.Name) {
			return nil, nil
		}
	}

	return []vm.InstanceInfo{*inst}, nil
}

// findExistingAppleInstance checks if an Apple Container instance already exists.
// Returns the profile name if found, empty string otherwise.
func findExistingAppleInstance() string {
	for name, c := range configmanager.LoadProfiles() {
		if c.Runtime == Name {
			return name
		}
	}
	return ""
}

// findAppleInstance finds the Apple Container instance (only one can exist).
func findAppleInstance() *vm.InstanceInfo {
	profileName := findExistingAppleInstance()
	if profileName == "" {
		return nil
	}

	// Check system status
	status := "Stopped"
	cmd := cli.Command(ContainerCommand, "system", "status")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if cmd.Run() == nil {
		status = "Running"
	}

	return &vm.InstanceInfo{
		Name:    profileName,
		Status:  status,
		Arch:    string(environment.HostArch()),
		CPU:     -1, // N/A for Apple Container
		Memory:  -1, // N/A for Apple Container
		Disk:    -1, // N/A for Apple Container
		Runtime: Name,
		Backend: string(vm.BackendApple),
	}
}

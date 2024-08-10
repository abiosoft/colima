package daemon

import (
	"context"
	"fmt"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/daemon/process/inotify"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/osutil"
)

// Manager handles running background processes.
type Manager interface {
	Start(context.Context, config.Config) error
	Stop(context.Context, config.Config) error
	Running(context.Context, config.Config) (Status, error)
	Dependencies(context.Context, config.Config) (deps process.Dependency, root bool)
}

type Status struct {
	// Parent process
	Running bool
	// Subprocesses
	Processes []processStatus
}
type processStatus struct {
	Name    string
	Running bool
	Error   error
}

// NewManager creates a new process manager.
func NewManager(host environment.HostActions) Manager {
	return &processManager{
		host: host,
	}
}

func CtxKey(s string) any { return struct{ key string }{key: s} }

var _ Manager = (*processManager)(nil)

type processManager struct {
	host environment.HostActions
}

func (l processManager) Dependencies(ctx context.Context, conf config.Config) (deps process.Dependency, root bool) {
	processes := processesFromConfig(conf)
	return process.Dependencies(processes...)
}

func (l processManager) init() error {
	// dependencies for network
	if err := fsutil.MkdirAll(process.Dir(), 0755); err != nil {
		return fmt.Errorf("error preparing vmnet: %w", err)
	}
	return nil
}

func (l processManager) Running(ctx context.Context, conf config.Config) (s Status, err error) {
	err = l.host.RunQuiet(osutil.Executable(), "daemon", "status", config.CurrentProfile().ShortName)
	if err != nil {
		return
	}
	s.Running = true

	ctx = context.WithValue(ctx, process.CtxKeyDaemon(), s.Running)

	for _, p := range processesFromConfig(conf) {
		pErr := p.Alive(ctx)
		s.Processes = append(s.Processes, processStatus{
			Name:    p.Name(),
			Running: pErr == nil,
			Error:   pErr,
		})
	}
	return
}

func (l processManager) Start(ctx context.Context, conf config.Config) error {
	_ = l.Stop(ctx, conf) // this is safe, nothing is done when not running

	if err := l.init(); err != nil {
		return fmt.Errorf("error preparing daemon directory: %w", err)
	}

	args := []string{osutil.Executable(), "daemon", "start", config.CurrentProfile().ShortName}

	if conf.Network.Address {
		args = append(args, "--vmnet")
	}
	if conf.MountINotify {
		args = append(args, "--inotify")
		args = append(args, "--inotify-runtime", conf.Runtime)
		for _, mount := range conf.MountsOrDefault() {
			p, err := util.CleanPath(mount.Location)
			if err != nil {
				return fmt.Errorf("error sanitising mount path for inotify: %w", err)
			}
			args = append(args, "--inotify-dir", p)
		}
	}

	if cli.Settings.Verbose {
		args = append(args, "--very-verbose")
	}

	host := l.host.WithDir(util.HomeDir())
	return host.RunQuiet(args...)
}
func (l processManager) Stop(ctx context.Context, conf config.Config) error {
	if s, err := l.Running(ctx, conf); err != nil || !s.Running {
		return nil
	}
	return l.host.RunQuiet(osutil.Executable(), "daemon", "stop", config.CurrentProfile().ShortName)
}

func processesFromConfig(conf config.Config) []process.Process {
	var processes []process.Process

	if conf.Network.Address {
		processes = append(processes, vmnet.New())
	}
	if conf.MountINotify {
		processes = append(processes, inotify.New())
	}

	return processes
}

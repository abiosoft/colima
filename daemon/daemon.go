package daemon

import (
	"context"
	"fmt"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/daemon/process/gvproxy"
	"github.com/abiosoft/colima/daemon/process/inotify"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/osutil"
)

// Manager handles running background processes.
type Manager interface {
	Start(context.Context) error
	Stop(context.Context) error
	Running(ctx context.Context) (Status, error)
	Dependencies(ctx context.Context) (deps process.Dependency, root bool)
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

func (l processManager) Dependencies(ctx context.Context) (deps process.Dependency, root bool) {
	processes := processesFromCtx(ctx)
	return process.Dependencies(processes...)
}

func (l processManager) init() error {
	// dependencies for network
	if err := fsutil.MkdirAll(process.Dir(), 0755); err != nil {
		return fmt.Errorf("error preparing vmnet: %w", err)
	}
	return nil
}

func (l processManager) Running(ctx context.Context) (s Status, err error) {
	err = l.host.RunQuiet(osutil.Executable(), "daemon", "status", config.CurrentProfile().ShortName)
	if err != nil {
		return
	}
	s.Running = true

	ctx = context.WithValue(ctx, process.CtxKeyDaemon(), s.Running)

	for _, p := range processesFromCtx(ctx) {
		pErr := p.Alive(ctx)
		s.Processes = append(s.Processes, processStatus{
			Name:    p.Name(),
			Running: pErr == nil,
			Error:   pErr,
		})
	}
	return
}

func (l processManager) Start(ctx context.Context) error {
	_ = l.Stop(ctx) // this is safe, nothing is done when not running

	if err := l.init(); err != nil {
		return fmt.Errorf("error preparing network directory: %w", err)
	}

	args := []string{osutil.Executable(), "daemon", "start", config.CurrentProfile().ShortName}
	opts := optsFromCtx(ctx)
	if opts.Vmnet {
		args = append(args, "--vmnet")
	}
	if opts.INotify {
		args = append(args, "--inotify")
	}
	if opts.GVProxy.Enabled {
		args = append(args, "--gvproxy")
		if len(opts.GVProxy.Hosts) > 0 {
			for host, ip := range opts.GVProxy.Hosts {
				args = append(args, "--gvproxy-hosts", host+"="+ip)
			}
		}
	}

	if cli.Settings.Verbose {
		args = append(args, "--very-verbose")
	}

	host := l.host.WithDir(util.HomeDir())
	return host.RunQuiet(args...)
}
func (l processManager) Stop(ctx context.Context) error {
	if s, err := l.Running(ctx); err != nil || !s.Running {
		return nil
	}
	return l.host.RunQuiet(osutil.Executable(), "daemon", "stop", config.CurrentProfile().ShortName)
}

func optsFromCtx(ctx context.Context) struct {
	Vmnet   bool
	GVProxy struct {
		Enabled bool
		Hosts   map[string]string
	}
	INotify bool
} {
	var opts = struct {
		Vmnet   bool
		GVProxy struct {
			Enabled bool
			Hosts   map[string]string
		}
		INotify bool
	}{}
	opts.Vmnet, _ = ctx.Value(CtxKey(vmnet.Name)).(bool)
	opts.GVProxy.Enabled, _ = ctx.Value(CtxKey(gvproxy.Name)).(bool)
	opts.GVProxy.Hosts, _ = ctx.Value(CtxKey(gvproxy.CtxKeyHosts)).(map[string]string)
	opts.INotify, _ = ctx.Value(CtxKey(inotify.Name)).(bool)

	return opts
}

func processesFromCtx(ctx context.Context) []process.Process {
	var processes []process.Process

	opts := optsFromCtx(ctx)
	if opts.Vmnet {
		processes = append(processes, vmnet.New())
	}
	if opts.GVProxy.Enabled {
		processes = append(processes, gvproxy.New(opts.GVProxy.Hosts))
	}
	if opts.INotify {
		processes = append(processes, inotify.New())
	}

	return processes
}

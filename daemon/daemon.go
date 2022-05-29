package daemon

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/daemon/process/gvproxy"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/environment"
	"github.com/sirupsen/logrus"
)

// Run runs the daemon with background processes.
// NOTE: this must be called from the program entrypoint with minimal intermediary logic
// due to the creation of the daemon.
func Run(ctx context.Context, processes ...process.Process) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(len(processes))

	for _, bg := range processes {
		go func(bg process.Process) {
			err := bg.Start(ctx)
			if err != nil {
				logrus.Error(fmt.Errorf("error starting %s: %w", bg.Name(), err))
				stop()
			}
			wg.Done()
		}(bg)
	}

	<-ctx.Done()
	logrus.Info("terminate signal received")

	wg.Wait()

	return ctx.Err()
}

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
	if err := os.MkdirAll(process.Dir(), 0755); err != nil {
		return fmt.Errorf("error preparing vmnet: %w", err)
	}
	return nil
}

func (l processManager) Running(ctx context.Context) (s Status, err error) {
	err = l.host.RunQuiet(os.Args[0], "daemon", "status", config.CurrentProfile().ShortName)
	if err != nil {
		return
	}
	s.Running = true

	for _, process := range processesFromCtx(ctx) {
		pErr := process.Alive(ctx)
		s.Processes = append(s.Processes, processStatus{
			Name:    process.Name(),
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

	args := []string{os.Args[0], "daemon", "start", config.CurrentProfile().ShortName}
	opts := optsFromCtx(ctx)
	if opts.Vmnet {
		args = append(args, "--vmnet")
	}
	if opts.GVProxy {
		args = append(args, "--gvproxy")
	}

	return l.host.RunQuiet(args...)
}
func (l processManager) Stop(ctx context.Context) error {
	if s, err := l.Running(ctx); err != nil || !s.Running {
		return nil
	}
	return l.host.RunQuiet(os.Args[0], "daemon", "stop", config.CurrentProfile().ShortName)
}

func optsFromCtx(ctx context.Context) struct {
	Vmnet   bool
	GVProxy bool
} {
	var opts = struct {
		Vmnet   bool
		GVProxy bool
	}{}
	opts.Vmnet, _ = ctx.Value(CtxKey(vmnet.Name())).(bool)
	opts.GVProxy, _ = ctx.Value(CtxKey(gvproxy.Name())).(bool)

	return opts
}

func processesFromCtx(ctx context.Context) []process.Process {
	var processes []process.Process

	opts := optsFromCtx(ctx)
	if opts.Vmnet {
		processes = append(processes, vmnet.New())
	}
	if opts.GVProxy {
		processes = append(processes, gvproxy.New())
	}

	return processes
}

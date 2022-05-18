package network

import (
	"context"
	"fmt"
	"os"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/gvproxy"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/vmnet"
)

// Manager handles networking between the host and the vm.
type Manager interface {
	Start(context.Context) error
	Stop(context.Context) error
	Running(ctx context.Context) (Status, error)
	Dependencies(ctx context.Context) (deps daemon.Dependency, root bool)
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

// NewManager creates a new network manager.
func NewManager(host environment.HostActions) Manager {
	return &limaNetworkManager{
		host: host,
	}
}

func CtxKey(s string) any { return struct{ key string }{key: s} }

var _ Manager = (*limaNetworkManager)(nil)

type limaNetworkManager struct {
	host environment.HostActions
}

func (l limaNetworkManager) Dependencies(ctx context.Context) (deps daemon.Dependency, root bool) {
	processes := processesFromCtx(ctx)
	return daemon.Dependencies(processes...)
}

func (l limaNetworkManager) init() error {
	// dependencies for network
	if err := os.MkdirAll(daemon.Dir(), 0755); err != nil {
		return fmt.Errorf("error preparing vmnet: %w", err)
	}
	return nil
}

func (l limaNetworkManager) Running(ctx context.Context) (s Status, err error) {
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

func (l limaNetworkManager) Start(ctx context.Context) error {
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
func (l limaNetworkManager) Stop(ctx context.Context) error {
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

func processesFromCtx(ctx context.Context) []daemon.Process {
	var processes []daemon.Process

	opts := optsFromCtx(ctx)
	if opts.Vmnet {
		processes = append(processes, vmnet.New())
	}
	if opts.GVProxy {
		processes = append(processes, gvproxy.New())
	}

	return processes
}

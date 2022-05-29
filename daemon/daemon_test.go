package daemon

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/abiosoft/colima/daemon/process"
)

func TestStart(t *testing.T) {
	var addresses = []string{
		"localhost",
		"127.0.0.1",
	}

	var processes []process.Process
	for _, add := range addresses {
		processes = append(processes, &pinger{address: add})
	}

	timeout := time.Second * 30
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// start the processes
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, processes...)
	}()

	cancel()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Error(err)
		}
	case err := <-done:
		t.Error(err)
	}

}

var _ process.Process = (*pinger)(nil)

type pinger struct {
	address string
}

func (p pinger) Alive(ctx context.Context) error {
	return nil
}

// Name implements BgProcess
func (pinger) Name() string { return "pinger" }

// Start implements BgProcess
func (p *pinger) Start(ctx context.Context) error {
	return p.run(ctx, "ping", "-c10", p.address)
}

// Start implements BgProcess
func (p *pinger) Dependencies() ([]process.Dependency, bool) { return nil, false }

func (p *pinger) run(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

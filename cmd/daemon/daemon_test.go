package daemon

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/abiosoft/colima/environment/vm/lima/network/daemon"
)

var testDir string

func setDir(t *testing.T) {
	if testDir == "" {
		testDir = t.TempDir()
	}
	dir = func() string { return testDir }
}

func TestStart(t *testing.T) {
	setDir(t)
	info := Info()

	var addresses = []string{
		"localhost",
		"127.0.0.1",
	}

	t.Log("pidfile", info.PidFile)

	var processes []daemon.Process
	for _, add := range addresses {
		processes = append(processes, &pinger{address: add})
	}

	timeout := time.Second * 5
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// start the processes
	if err := start(ctx, processes); err != nil {
		t.Fatal(err)
	}
	t.Log("start successful")

	{
	loop:
		for {
			select {
			case <-ctx.Done():
				t.Skipf("daemon not supported: %v", ctx.Err())
			default:
				if p, err := os.ReadFile(info.PidFile); err == nil && len(p) > 0 {
					break loop
				}
				time.Sleep(1 * time.Second)
			}
		}
	}

	// verify the processes are running
	if err := status(); err != nil {
		t.Error(err)
		return
	}

	// stop the processes
	if err := stop(ctx); err != nil {
		t.Error(err)
	}

	// verify the processes are no longer running
	if err := status(); err == nil {
		t.Errorf("process with pidFile %s is still running", info.PidFile)
		return
	}

}

var _ daemon.Process = (*pinger)(nil)

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
func (p *pinger) Dependencies() ([]daemon.Dependency, bool) { return nil, false }

func (p *pinger) run(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

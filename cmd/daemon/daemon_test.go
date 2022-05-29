package daemon

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/abiosoft/colima/daemon/process"
)

var testDir string

func setDir(t *testing.T) {
	if testDir == "" {
		testDir = t.TempDir()
	}
	dir = func() string { return testDir }
}

func getProcesses() []process.Process {
	var addresses = []string{
		"localhost",
		"127.0.0.1",
	}

	var processes []process.Process
	for _, add := range addresses {
		processes = append(processes, &pinger{address: add})
	}

	return processes
}

func TestStart(t *testing.T) {
	setDir(t)
	info := Info()

	processes := getProcesses()

	t.Log("pidfile", info.PidFile)

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
				} else if err != nil {
					t.Logf("encountered err: %v", err)
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

func TestRunProcesses(t *testing.T) {
	processes := getProcesses()

	timeout := time.Second * 5
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// start the processes
	done := make(chan error, 1)
	go func() {
		done <- RunProcesses(ctx, processes...)
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/util/fsutil"
	godaemon "github.com/sevlyar/go-daemon"
	"github.com/sirupsen/logrus"
)

var dir = process.Dir

// daemonize creates the daemon and returns if this is a child process
func daemonize() (ctx *godaemon.Context, child bool, err error) {
	dir := dir()
	if err := fsutil.MkdirAll(dir, 0755); err != nil {
		return nil, false, fmt.Errorf("cannot make dir: %w", err)
	}

	info := Info()

	ctx = &godaemon.Context{
		PidFileName: info.PidFile,
		PidFilePerm: 0644,
		LogFileName: info.LogFile,
		LogFilePerm: 0644,
	}

	d, err := ctx.Reborn()
	if err != nil {
		return ctx, false, fmt.Errorf("error starting daemon: %w", err)
	}
	if d != nil {
		return ctx, false, nil
	}

	logrus.Info("- - - - - - - - - - - - - - -")
	logrus.Info("daemon started by colima")
	logrus.Infof("Run `/usr/bin/pkill -F %s` to kill the daemon", info.PidFile)

	return ctx, true, nil
}

func start(ctx context.Context, processes []process.Process) error {
	if status() == nil {
		logrus.Info("daemon already running, startup ignored")
		return nil
	}

	{
		ctx, child, err := daemonize()
		if err != nil {
			return err
		}

		if ctx != nil {
			defer func() {
				_ = ctx.Release()
			}()
		}

		if !child {
			return nil
		}
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return RunProcesses(ctx, processes...)
}

func stop(ctx context.Context) error {
	if status() != nil {
		// not running
		return nil
	}

	info := Info()

	if err := cli.CommandInteractive("/usr/bin/pkill", "-F", info.PidFile).Run(); err != nil {
		return fmt.Errorf("error sending sigterm to daemon: %w", err)
	}

	logrus.Info("waiting for process to terminate")

	for {
		alive := status() == nil
		if !alive {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(time.Second * 1)
		}
	}

}

func status() error {
	info := Info()
	if _, err := os.Stat(info.PidFile); err != nil {
		return fmt.Errorf("pid file not found: %w", err)
	}

	// check if process is actually running
	p, err := os.ReadFile(info.PidFile)
	if err != nil {
		return fmt.Errorf("error reading pid file: %w", err)
	}
	pid, _ := strconv.Atoi(string(p))
	if pid == 0 {
		return fmt.Errorf("invalid pid: %v", string(p))
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %v", err)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("process signal(0) returned error: %w", err)
	}

	return nil
}

const (
	pidFileName = "daemon.pid"
	logFileName = "daemon.log"
)

func Info() struct {
	PidFile string
	LogFile string
} {
	dir := dir()
	return struct {
		PidFile string
		LogFile string
	}{
		PidFile: filepath.Join(dir, pidFileName),
		LogFile: filepath.Join(dir, logFileName),
	}
}

// Run runs the daemon with background processes.
// NOTE: this must be called from the program entrypoint with minimal intermediary logic
// due to the creation of the daemon.
func RunProcesses(ctx context.Context, processes ...process.Process) error {
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

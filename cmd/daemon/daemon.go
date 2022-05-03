package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon"
	godaemon "github.com/sevlyar/go-daemon"
	"github.com/sirupsen/logrus"
)

var dir = daemon.Dir

// daemonize creates the daemon and returns if this is a child process
func daemonize() (ctx *godaemon.Context, child bool, err error) {
	dir := dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
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
	logrus.Infof("Run `pkill -F %s` to kill the daemon", info.PidFile)

	return ctx, true, nil
}

func start(ctx context.Context, processes []daemon.Process) error {
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

	return daemon.Run(ctx, processes...)
}

func stop(ctx context.Context) error {
	if status() != nil {
		// not running
		return nil
	}

	info := Info()

	if err := cli.CommandInteractive("pkill", "-F", info.PidFile).Run(); err != nil {
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

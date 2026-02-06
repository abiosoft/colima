package socktainer

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/sirupsen/logrus"
)

// Name is the name of the socktainer process.
const Name = "socktainer"

// socktainerDir is the directory name for socktainer installation.
const socktainerDir = "_socktainer"

// BinPath returns the absolute path to the socktainer binary.
func BinPath() string {
	return filepath.Join(config.Dir(), socktainerDir, "bin", Name)
}

// socketDir is the directory where socktainer creates its socket.
// This is a fixed path managed by socktainer itself.
const socketDir = ".socktainer"

// socketName is the name of the socket file.
const socketName = "container.sock"

var _ process.Process = (*socktainerProcess)(nil)

// New creates a new socktainer process.
func New() process.Process {
	return &socktainerProcess{}
}

type socktainerProcess struct{}

// Name implements process.Process.
func (*socktainerProcess) Name() string { return Name }

// Alive implements process.Process.
func (*socktainerProcess) Alive(ctx context.Context) error {
	socketFile := SocketFile()

	// Check if socket file exists and is accessible
	if _, err := os.Stat(socketFile); err != nil {
		return fmt.Errorf("socktainer socket not found: %w", err)
	}

	// Try to connect to the socket
	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		return fmt.Errorf("socktainer socket not accessible: %w", err)
	}
	if err := conn.Close(); err != nil {
		logrus.Debugln(fmt.Errorf("error closing socket connection: %w", err))
	}

	return nil
}

// Start implements process.Process.
// Socktainer is a blocking process that runs until the context is cancelled.
// It automatically restarts on crash or error exit.
func (s *socktainerProcess) Start(ctx context.Context) error {
	for {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		err := s.runOnce(ctx)
		if err == nil {
			// Clean exit, check if we should restart
			select {
			case <-ctx.Done():
				return nil
			default:
				// Process exited cleanly but context not cancelled, restart
				logrus.Debugln("socktainer exited, restarting...")
			}
		} else {
			// Error exit, check if context is cancelled
			select {
			case <-ctx.Done():
				return nil
			default:
				// Context not cancelled, log and restart
				logrus.Warnln(fmt.Errorf("socktainer crashed, restarting: %w", err))
			}
		}

		// Small delay before restart to prevent rapid restart loops
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}

// runOnce starts socktainer and waits for it to exit or context cancellation.
func (s *socktainerProcess) runOnce(ctx context.Context) error {
	done := make(chan error, 1)

	// Start socktainer process - it manages its own socket at ~/.socktainer/container.sock
	// Use non-interactive command as socktainer is a background daemon
	command := cli.Command(BinPath())

	if cli.Settings.Verbose {
		command.Env = append(command.Env, os.Environ()...)
		command.Env = append(command.Env, "DEBUG=1")
	}

	// Start the command and write PID to file
	if err := command.Start(); err != nil {
		return fmt.Errorf("error starting socktainer: %w", err)
	}

	// Write PID to file
	if err := writePidFile(command.Process.Pid); err != nil {
		logrus.Warnln(fmt.Errorf("error writing socktainer pid file: %w", err))
	}

	go func() {
		done <- command.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled, kill the process
		_ = command.Process.Kill()
		return nil
	case err := <-done:
		return err
	}
}

// Dependencies implements process.Process.
func (s *socktainerProcess) Dependencies() (deps []process.Dependency, root bool) {
	return []process.Dependency{
		socktainerBinary{},
	}, false
}

// SocketFile returns the path to the socktainer socket file.
// The socket is always at $HOME/.socktainer/container.sock.
func SocketFile() string {
	return filepath.Join(util.HomeDir(), socketDir, socketName)
}

// Socket returns the socket as osutil.Socket.
func Socket() osutil.Socket {
	return osutil.Socket(SocketFile())
}

// PidFile returns the path to the socktainer PID file.
func PidFile() string {
	return filepath.Join(process.Dir(), "socktainer.pid")
}

// writePidFile writes the PID to the PID file.
func writePidFile(pid int) error {
	pidFile := PidFile()
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// Stop terminates the socktainer process using the PID file.
func Stop() error {
	pidFile := PidFile()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No PID file, nothing to stop
		}
		return fmt.Errorf("error reading socktainer pid file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("error parsing socktainer pid: %w", err)
	}

	// Send SIGTERM to the process
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding socktainer process: %w", err)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil {
		// Process might already be dead
		if err != os.ErrProcessDone {
			logrus.Debugln(fmt.Errorf("error sending SIGTERM to socktainer: %w", err))
		}
	}

	// Remove PID file
	_ = os.Remove(pidFile)

	return nil
}

package socktainer

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/sirupsen/logrus"
)

// Name is the name of the socktainer process.
const Name = "socktainer"

// Command is the socktainer binary name.
const Command = Name

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
func (s *socktainerProcess) Start(ctx context.Context) error {
	done := make(chan error, 1)

	go func() {
		// Start socktainer process - it manages its own socket at ~/.socktainer/container.sock
		command := cli.CommandInteractive(Command)

		if cli.Settings.Verbose {
			command.Env = append(command.Env, os.Environ()...)
			command.Env = append(command.Env, "DEBUG=1")
		}

		done <- command.Run()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled, socktainer will be terminated by the daemon manager
		return nil
	case err := <-done:
		if err != nil {
			return fmt.Errorf("error running socktainer: %w", err)
		}
	}

	return nil
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

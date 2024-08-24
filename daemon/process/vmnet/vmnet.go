package vmnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/sirupsen/logrus"
)

const Name = "vmnet"

const (
	SubProcessEnvVar = "COLIMA_VMNET"

	NetGateway = "192.168.106.1"
	NetDHCPEnd = "192.168.106.254"
)

var _ process.Process = (*vmnetProcess)(nil)

func New() process.Process { return &vmnetProcess{} }

type vmnetProcess struct{}

func (*vmnetProcess) Alive(ctx context.Context) error {
	info := Info()
	pidFile := info.PidFile
	socketFile := info.Socket.File()

	if _, err := os.Stat(pidFile); err == nil {
		cmd := exec.CommandContext(ctx, "sudo", "/usr/bin/pkill", "-0", "-F", pidFile)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error checking vmnet process: %w", err)
		}
	}

	if _, err := os.Stat(socketFile); err != nil {
		return fmt.Errorf("vmnet socket file not found error: %w", err)
	}
	if n, err := net.Dial("unix", socketFile); err != nil {
		return fmt.Errorf("vmnet socket file error: %w", err)
	} else {
		if err := n.Close(); err != nil {
			logrus.Debugln(fmt.Errorf("error closing ping socket connection: %w", err))
		}
	}

	return nil
}

// Name implements process.BgProcess
func (*vmnetProcess) Name() string { return Name }

// Start implements process.BgProcess
func (*vmnetProcess) Start(ctx context.Context) error {
	info := Info()
	socket := info.Socket.File()
	pid := info.PidFile

	// delete existing sockets if exist
	// errors ignored on purpose
	_ = forceDeleteFileIfExists(socket)

	done := make(chan error, 1)

	go func() {
		// rootfully start the vmnet daemon
		command := cli.CommandInteractive("sudo", BinaryPath,
			"--vmnet-mode", "shared",
			"--socket-group", "staff",
			"--vmnet-gateway", NetGateway,
			"--vmnet-dhcp-end", NetDHCPEnd,
			"--pidfile", pid,
			socket,
		)

		if cli.Settings.Verbose {
			command.Env = append(command.Env, os.Environ()...)
			command.Env = append(command.Env, "DEBUG=1")
		}

		done <- command.Run()
	}()

	select {
	case <-ctx.Done():
		if err := stop(pid); err != nil {
			return fmt.Errorf("error stopping vmnet: %w", err)
		}
	case err := <-done:
		if err != nil {
			return fmt.Errorf("error running vmnet: %w", err)
		}
	}

	return nil
}

func (vmnetProcess) Dependencies() (deps []process.Dependency, root bool) {
	return []process.Dependency{
		sudoerFile{},
		vmnetFile{},
		vmnetRunDir{},
	}, true
}

func stop(pidFile string) error {
	// rootfully kill the vmnet process.
	// process is only assumed alive if the pidfile exists
	if _, err := os.Stat(pidFile); err == nil {
		if err := cli.CommandInteractive("sudo", "/usr/bin/pkill", "-F", pidFile).Run(); err != nil {
			return fmt.Errorf("error killing vmnet process: %w", err)
		}
	}

	return nil
}

func forceDeleteFileIfExists(name string) error {
	if stat, err := os.Stat(name); err == nil && !stat.IsDir() {
		return os.Remove(name)
	}
	return nil
}

func Info() struct {
	PidFile string
	Socket  osutil.Socket
} {
	return struct {
		PidFile string
		Socket  osutil.Socket
	}{
		PidFile: filepath.Join(runDir(), "vmnet-"+config.CurrentProfile().ShortName+".pid"),
		Socket:  osutil.Socket(filepath.Join(process.Dir(), "vmnet.sock")),
	}
}

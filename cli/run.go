package cli

import (
	"os"
	"os/exec"
)

var stdout = os.Stdout
var stderr = os.Stderr

func Stdout(file *os.File) { stdout = file }
func Stderr(file *os.File) { stderr = file }

// 	Run runs command
func Run(command string, args ...string) error {
	return NewCommand(command, args...).Run()
}

// NewCommand creates a new command.
func NewCommand(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

// Controller is the controller for performing actions on the host and guest.
type Controller interface {
	Host() HostActions
	Guest() GuestActions
}

// RunAction runs commands.
type RunAction interface {
	// Run runs command
	Run(args ...string) error
}

// HostActions are actions performed on the host.
type HostActions interface {
	RunAction
}

// GuestActions are actions performed on the guest i.e. VM.
type GuestActions interface {
	RunAction
	// Start starts up the VM
	Start() error
	// Stop shuts down the VM
	Stop() error
}

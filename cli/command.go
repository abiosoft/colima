package cli

import (
	"os"
	"os/exec"
)

var d commandRunner = &defaultCommandRunner{
	stdout: os.Stdout,
	stderr: os.Stderr,
}

// Stdout sets the stdout for commands.
func Stdout(file *os.File) { d.Stdout(file) }

// Stderr sets the stderr for commands.
func Stderr(file *os.File) { d.Stdout(file) }

// Command creates a new command.
func Command(command string, args ...string) *exec.Cmd { return d.Command(command, args...) }

// CommandInteractive creates a new interactive command.
func CommandInteractive(command string, args ...string) *exec.Cmd {
	return d.CommandInteractive(command, args...)
}

type commandRunner interface {
	Stdout(file *os.File)
	Stderr(file *os.File)
	Command(command string, args ...string) *exec.Cmd
	CommandInteractive(command string, args ...string) *exec.Cmd
}

var _ commandRunner = (*defaultCommandRunner)(nil)

type defaultCommandRunner struct {
	stdout *os.File
	stderr *os.File
}

func (d *defaultCommandRunner) Stdout(file *os.File) { d.stdout = file }

func (d *defaultCommandRunner) Stderr(file *os.File) { d.stderr = file }

func (d defaultCommandRunner) Command(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = d.stdout
	cmd.Stderr = d.stderr
	return cmd
}

func (d defaultCommandRunner) CommandInteractive(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

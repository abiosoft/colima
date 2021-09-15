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

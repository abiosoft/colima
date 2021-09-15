package runner

import (
	"os"
	"os/exec"
)

var stdout = os.Stdout
var stderr = os.Stderr

// Stdout sets the stdout.
func Stdout(file *os.File) { stdout = file }

// Stderr sets the stderr.
func Stderr(file *os.File) { stderr = file }

// Command creates a new command.
func Command(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

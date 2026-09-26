package native

import (
	"fmt"
	"io"
	"os"
)

// Run runs a command directly on the host.
// Unlike Lima's shell.go which prepends "lima" (SSH into VM),
// native mode executes commands directly.
func (n nativeVM) Run(args ...string) error {
	return n.host.Run(args...)
}

// RunQuiet runs a command on the host whilst suppressing output.
func (n nativeVM) RunQuiet(args ...string) error {
	return n.host.RunQuiet(args...)
}

// RunOutput runs a command on the host and returns its output.
func (n nativeVM) RunOutput(args ...string) (string, error) {
	return n.host.RunOutput(args...)
}

// RunInteractive runs a command on the host interactively.
func (n nativeVM) RunInteractive(args ...string) error {
	return n.host.RunInteractive(args...)
}

// RunWith runs a command on the host with custom stdin/stdout.
func (n nativeVM) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	return n.host.RunWith(stdin, stdout, args...)
}

// SSH opens an interactive session on the host.
// In native mode there is no VM to SSH into, so we either execute the
// provided command directly or open a shell.
func (n nativeVM) SSH(workingDir string, args ...string) error {
	if len(args) > 0 {
		// Execute the command directly on the host
		if workingDir != "" {
			return n.host.WithDir(workingDir).RunInteractive(args...)
		}
		return n.host.RunInteractive(args...)
	}

	// No args: open an interactive shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	if workingDir != "" {
		return n.host.RunInteractive("sh", "-c",
			fmt.Sprintf("cd %s && exec %s", workingDir, shell))
	}

	return n.host.RunInteractive(shell)
}

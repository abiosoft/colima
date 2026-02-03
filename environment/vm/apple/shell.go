package apple

import (
	"fmt"
	"io"
)

// Run runs a command in the container.
func (a appleVM) Run(args ...string) error {
	return fmt.Errorf("run is not supported for Apple Container runtime")
}

// RunQuiet runs a command in the container whilst suppressing the output.
func (a appleVM) RunQuiet(args ...string) error {
	return fmt.Errorf("run is not supported for Apple Container runtime")
}

// RunOutput runs a command in the container and returns its output.
func (a appleVM) RunOutput(args ...string) (out string, err error) {
	return "", fmt.Errorf("run is not supported for Apple Container runtime")
}

// RunInteractive runs a command interactively in the container.
func (a appleVM) RunInteractive(args ...string) error {
	return fmt.Errorf("run is not supported for Apple Container runtime")
}

// RunWith runs a command with stdin and stdout in the container.
func (a appleVM) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	return fmt.Errorf("run is not supported for Apple Container runtime")
}

// SSH is not supported for Apple Container as there is no VM to SSH into.
func (a appleVM) SSH(workingDir string, args ...string) error {
	return fmt.Errorf("ssh is not supported for Apple Container runtime")
}

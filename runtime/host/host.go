package host

import (
	"errors"
	"fmt"
	"github.com/abiosoft/colima/runner"
	"github.com/abiosoft/colima/runtime"
	"os"
	"strings"
)

// Runtime is the host runtime.
type Runtime interface {
	runtime.HostActions
}

// New creates a new host runtime using env as environment variables.
func New() Runtime {
	return &hostRuntime{}
}

var _ Runtime = (*hostRuntime)(nil)

type hostRuntime struct {
	env []string
}

func (h hostRuntime) WithEnv(env []string) runtime.HostActions {
	var newHost hostRuntime
	// use current and new env vars
	newHost.env = append(newHost.env, h.env...)
	newHost.env = append(newHost.env, env...)
	return newHost
}

func (h hostRuntime) Run(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := runner.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	return cmd.Run()
}

// IsInstalled checks if dependencies are installed.
func IsInstalled(dependencies runtime.Dependencies) error {
	var missing []string
	check := func(p string) error {
		return runner.Command("command", "-v", p).Run()
	}
	for _, p := range dependencies.Dependencies() {
		if check(p) != nil {
			missing = append(missing, p)
		}
	}
	return fmt.Errorf("%s not found, run 'brew install %s' to install", strings.Join(missing, ", "), strings.Join(missing, " "))
}

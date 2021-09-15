package host

import (
	"errors"
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/runtime"
	"strings"
)

// Runtime is the host runtime.
type Runtime interface {
	runtime.HostActions
}

// New creates a new host runtime using env as environment variables.
func New(env []string) Runtime {
	return &host{env: env}
}

var _ Runtime = (*host)(nil)

type host struct {
	env []string
}

func (h host) Run(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.NewCommand(args[0], args[1:]...)
	cmd.Env = append(cmd.Env, h.env...)
	return cmd.Run()
}

// IsInstalled checks if dependencies are installed.
func (h host) IsInstalled(dependencies runtime.Dependencies) error {
	var missing []string
	check := func(p string) error {
		return h.Run("command", "-v", p)
	}
	for _, p := range dependencies.Dependencies() {
		if check(p) != nil {
			missing = append(missing, p)
		}
	}
	return fmt.Errorf("%s not found, run 'brew install %s' to install", strings.Join(missing, ", "), strings.Join(missing, " "))
}

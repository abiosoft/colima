package host

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/runtime"
	"os"
	"strings"
)

// Host is the host runtime.
type Host interface {
	runtime.HostActions
}

// New creates a new host runtime using env as environment variables.
func New() Host {
	return &hostRuntime{}
}

var _ Host = (*hostRuntime)(nil)

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
	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	return cmd.Run()
}

func (h hostRuntime) RunOutput(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("args not specified")
	}

	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)

	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func (h hostRuntime) RunInteractive(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.CommandInteractive(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	return cmd.Run()
}

// IsInstalled checks if dependencies are installed.
func IsInstalled(dependencies runtime.Dependencies) error {
	var missing []string
	check := func(p string) error {
		return cli.Command("command", "-v", p).Run()
	}
	for _, p := range dependencies.Dependencies() {
		if check(p) != nil {
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%s not found, run 'brew install %s' to install", strings.Join(missing, ", "), strings.Join(missing, " "))
	}

	return nil
}

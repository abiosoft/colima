package host

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/terminal"
)

// New creates a new host environment using env as environment variables.
func New(verbose bool) environment.Host {
	return &hostEnv{verbose: verbose}
}

var _ environment.Host = (*hostEnv)(nil)

type hostEnv struct {
	env     []string
	verbose bool
}

func (h hostEnv) WithEnv(env ...string) environment.HostActions {
	var newHost hostEnv
	// set verbose flag
	newHost.verbose = h.verbose
	// use current and new env vars
	newHost.env = append(newHost.env, h.env...)
	newHost.env = append(newHost.env, env...)
	return newHost
}

func (h hostEnv) Run(args ...string) error {
	var out io.WriteCloser
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)

	if h.verbose {
		out = os.Stdout
	} else {
		out = terminal.NewVerboseWriter(4)
		defer out.Close()
	}

	cmd.Stdout = out
	cmd.Stderr = out

	return cmd.Run()
}

func (h hostEnv) RunQuiet(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

func (h hostEnv) RunOutput(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("args not specified")
	}

	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func (h hostEnv) RunInteractive(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.CommandInteractive(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	return cmd.Run()
}

func (h hostEnv) Env(s string) string {
	return os.Getenv(s)
}

func (h hostEnv) Read(fileName string) (string, error) {
	b, err := os.ReadFile(fileName)
	return string(b), err
}

func (h hostEnv) Write(fileName, body string) error {
	return os.WriteFile(fileName, []byte(body), 0644)
}

func (h hostEnv) Stat(fileName string) (os.FileInfo, error) {
	return os.Stat(fileName)
}

// IsInstalled checks if dependencies are installed.
func IsInstalled(dependencies environment.Dependencies) error {
	var missing []string
	check := func(p string) error {
		cmd := cli.Command("command", "-v", p)
		cmd.Stderr = nil
		cmd.Stdout = nil
		return cmd.Run()
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

package host

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/abiosoft/colima/util/terminal"
	"os"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

// New creates a new host environment using env as environment variables
func New() environment.Host {
	return &hostEnv{}
}

var _ environment.Host = (*hostEnv)(nil)

type hostEnv struct {
	env []string
}

func (h hostEnv) WithEnv(env ...string) environment.HostActions {
	var newHost hostEnv
	// use current and new env vars
	newHost.env = append(newHost.env, h.env...)
	newHost.env = append(newHost.env, env...)
	return newHost
}

func (h hostEnv) Run(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)

	lineHeight := 6
	if cli.Settings.Verbose {
		lineHeight = -1 // disable scrolling
	}

	out := terminal.NewVerboseWriter(lineHeight)
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Run()
	if err == nil {
		return out.Close()
	}
	return err
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

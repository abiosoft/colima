package host

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/util/terminal"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

// New creates a new host environment.
func New() environment.Host {
	return &hostEnv{}
}

var _ environment.Host = (*hostEnv)(nil)

type hostEnv struct {
	env []string
	dir string // working directory
}

func (h hostEnv) clone() hostEnv {
	var newHost hostEnv
	newHost.env = append(newHost.env, h.env...)
	newHost.dir = h.dir
	return newHost
}

func (h hostEnv) WithEnv(env ...string) environment.HostActions {
	newHost := h.clone()
	// append new env vars
	newHost.env = append(newHost.env, env...)
	return newHost
}

func (h hostEnv) WithDir(dir string) environment.HostActions {
	newHost := h.clone()
	newHost.dir = dir
	return newHost
}

func (h hostEnv) Run(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	if h.dir != "" {
		cmd.Dir = h.dir
	}

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
	if h.dir != "" {
		cmd.Dir = h.dir
	}

	var errBuf bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return errCmd(cmd.Args, errBuf, err)
	}

	return nil
}

func (h hostEnv) RunOutput(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("args not specified")
	}

	cmd := cli.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	if h.dir != "" {
		cmd.Dir = h.dir
	}

	var buf, errBuf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", errCmd(cmd.Args, errBuf, err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func errCmd(args []string, stderr bytes.Buffer, err error) error {
	// this is going to be part of a log output,
	// reading the first line of the error should suffice
	output, _ := stderr.ReadString('\n')
	if len(output) > 0 {
		output = output[:len(output)-1]
	}
	return fmt.Errorf("error running %v, output: %s, err: %s", args, strconv.Quote(output), strconv.Quote(err.Error()))
}

func (h hostEnv) RunInteractive(args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.CommandInteractive(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	if h.dir != "" {
		cmd.Dir = h.dir
	}
	return cmd.Run()
}

func (h hostEnv) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	if len(args) == 0 {
		return errors.New("args not specified")
	}
	cmd := cli.CommandInteractive(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), h.env...)
	if h.dir != "" {
		cmd.Dir = h.dir
	}

	cmd.Stdin = stdin
	cmd.Stdout = stdout

	var buf bytes.Buffer
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return errCmd(cmd.Args, buf, err)
	}

	return nil
}

func (h hostEnv) Env(s string) string {
	return os.Getenv(s)
}

func (h hostEnv) Read(fileName string) (string, error) {
	b, err := os.ReadFile(fileName)
	return string(b), err
}

func (h hostEnv) Write(fileName string, body []byte) error {
	return os.WriteFile(fileName, body, 0644)
}

func (h hostEnv) Stat(fileName string) (os.FileInfo, error) {
	return os.Stat(fileName)
}

// IsInstalled checks if dependencies are installed.
func IsInstalled(dependencies environment.Dependencies) error {
	var missing []string
	check := func(p string) error {
		_, err := exec.LookPath(p)
		return err
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

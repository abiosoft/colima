package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var runner commandRunner = &defaultCommandRunner{
	stdout: os.Stdout,
	stderr: os.Stderr,
}

// DryRun toggles the state of the command runner. If true, commands are only printed to the console
// without execution.
func DryRun(d bool) {
	if d {
		runner = dryRunCommandRunner{}
		return
	}
	runner = &defaultCommandRunner{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// Stdout sets the stdout for commands.
func Stdout(file io.Writer) { runner.Stdout(file) }

// Stderr sets the stderr for commands.
func Stderr(file io.Writer) { runner.Stderr(file) }

// Command creates a new command.
func Command(command string, args ...string) *exec.Cmd { return runner.Command(command, args...) }

// CommandInteractive creates a new interactive command.
func CommandInteractive(command string, args ...string) *exec.Cmd {
	return runner.CommandInteractive(command, args...)
}

type commandRunner interface {
	Stdout(io.Writer)
	Stderr(writer io.Writer)
	Command(command string, args ...string) *exec.Cmd
	CommandInteractive(command string, args ...string) *exec.Cmd
}

var _ commandRunner = (*defaultCommandRunner)(nil)

type defaultCommandRunner struct {
	stdout io.Writer
	stderr io.Writer
}

func (d *defaultCommandRunner) Stdout(file io.Writer) { d.stdout = file }

func (d *defaultCommandRunner) Stderr(file io.Writer) { d.stderr = file }

func (d defaultCommandRunner) Command(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = d.stdout
	cmd.Stderr = d.stderr
	return cmd
}

func (d defaultCommandRunner) CommandInteractive(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

var _ commandRunner = (*dryRunCommandRunner)(nil)

type dryRunCommandRunner struct{}

func (d dryRunCommandRunner) Stdout(io.Writer) {}

func (d dryRunCommandRunner) Stderr(io.Writer) {}

func (d dryRunCommandRunner) Command(command string, args ...string) *exec.Cmd {
	d.printArgs("run:", command, args...)
	return exec.Command("echo")
}

func (d dryRunCommandRunner) CommandInteractive(command string, args ...string) *exec.Cmd {
	d.printArgs("interactive run:", command, args...)
	return exec.Command("echo")
}
func (d dryRunCommandRunner) printArgs(prefix, command string, args ...string) {
	var str []string
	str = append(str, prefix, strconv.Quote(command))
	for _, arg := range args {
		str = append(str, strconv.Quote(arg))
	}
	fmt.Println(strings.Join(str, " "))
}

// Prompt prompts for input with a question. It returns true only if answer is y or Y.
func Prompt(question string) bool {
	fmt.Print(question)
	fmt.Print("? [y/N] ")

	var answer string
	_, _ = fmt.Scanln(&answer)

	if answer == "" {
		return false
	}

	return answer[0] == 'Y' || answer[0] == 'y'
}

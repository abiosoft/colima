package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var runner commandRunner = &defaultCommandRunner{}

// Settings is global cli settings
var Settings = struct {
	// Verbose toggles verbose output for commands.
	Verbose bool
}{}

// Command creates a new command.
func Command(command string, args ...string) *exec.Cmd { return runner.Command(command, args...) }

// CommandInteractive creates a new interactive command.
func CommandInteractive(command string, args ...string) *exec.Cmd {
	return runner.CommandInteractive(command, args...)
}

type commandRunner interface {
	Command(command string, args ...string) *exec.Cmd
	CommandInteractive(command string, args ...string) *exec.Cmd
}

var _ commandRunner = (*defaultCommandRunner)(nil)

type defaultCommandRunner struct{}

func (d defaultCommandRunner) Command(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Trace("cmd ", quotedArgs(cmd.Args))

	return cmd
}

func (d defaultCommandRunner) CommandInteractive(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Trace("cmd int ", quotedArgs(cmd.Args))

	return cmd
}

func quotedArgs(args []string) string {
	var q []string
	for _, s := range args {
		q = append(q, strconv.Quote(s))
	}
	return fmt.Sprintf("%v", q)
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

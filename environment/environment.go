package environment

import (
	"os"

	"github.com/abiosoft/colima/config"
)

type runActions interface {
	// Run runs command
	Run(args ...string) error
	// RunQuiet runs command whilst suppressing the output.
	// Useful for commands that only the exit code matters.
	RunQuiet(args ...string) error
	// RunOutput runs command and returns its output.
	RunOutput(args ...string) (string, error)
	// RunInteractive runs command interactively.
	RunInteractive(args ...string) error
}

type fileActions interface {
	Read(fileName string) (string, error)
	Write(fileName, body string) error
	Stat(fileName string) (os.FileInfo, error)
}

// HostActions are actions performed on the host.
type HostActions interface {
	runActions
	fileActions
	// WithEnv creates a new instance based on the current instance
	// with the specified environment variables.
	WithEnv(env ...string) HostActions
	// Env retrieves environment variable on the host.
	Env(string) string
	// Port finds a random, unused port on the host.
	Port() (int, error)
}

// GuestActions are actions performed on the guest i.e. VM.
type GuestActions interface {
	runActions
	// Start starts up the VM
	Start(config.Config) error
	// Stop shuts down the VM
	Stop(force bool) error
	// Restart restarts the VM
	Restart() error
	// Created returns if the VM has been previously created.
	Created() bool
	// Running returns if the VM is currently running.
	Running() bool
	// Env retrieves environment variable in the VM.
	Env(string) (string, error)
	// Get retrieves a configuration in the VM.
	Get(key string) string
	// Set sets configuration in the VM.
	Set(key, value string) error
	// User returns the username of the user in the VM.
	User() (string, error)
	// Arch returns the architecture of the VM.
	Arch() Arch
}

// Dependencies are dependencies that must exist on the host.
type Dependencies interface {
	// Dependencies are dependencies that must exist on the host.
	// TODO this may need to accommodate non-brew installable dependencies
	Dependencies() []string
}

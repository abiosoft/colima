package runtime

import "github.com/abiosoft/colima/config"

// runAction runs commands.
type runAction interface {
	// Run runs command
	Run(args ...string) error
	// RunOutput runs command and returns its output.
	RunOutput(args ...string) (string, error)
	// RunInteractive runs command interactively.
	RunInteractive(args ...string) error
}

// HostActions are actions performed on the host.
type HostActions interface {
	runAction
	// WithEnv creates a new instance based on the current instance
	// with the specified environment variables.
	WithEnv(env []string) HostActions
}

// GuestActions are actions performed on the guest i.e. VM.
type GuestActions interface {
	runAction
	// Start starts up the VM
	Start(config.Config) error
	// Stop shuts down the VM
	Stop() error
	// Created returns if the VM has been previously created.
	Created() bool
	// Running returns if the VM is currently running.
	Running() bool
	// Env retrieves environment variable in the VM.
	Env(string) (string, error)
}

// Dependencies are dependencies that must exist on the host.
type Dependencies interface {
	// Dependencies are dependencies that must exist on the host.
	// TODO this may need to accommodate non-brew installable dependencies
	Dependencies() []string
}

package runtime

// runAction runs commands.
type runAction interface {
	// Run runs command
	Run(args ...string) error
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
	Start() error
	// Stop shuts down the VM
	Stop() error
}

// Dependencies are dependencies that must exist on the host.
type Dependencies interface {
	// Dependencies are dependencies that must exist on the host.
	// TODO this may need to accommodate non-brew installable dependencies
	Dependencies() []string
}

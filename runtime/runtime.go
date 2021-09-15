package runtime

// Controller is the controller for performing actions on the host and guest.
type Controller interface {
	Host() HostActions
	Guest() GuestActions
}

// RunAction runs commands.
type RunAction interface {
	// Run runs command
	Run(args ...string) error
}

// HostActions are actions performed on the host.
type HostActions interface {
	RunAction
	IsInstalled(Dependencies) error
}

// GuestActions are actions performed on the guest i.e. VM.
type GuestActions interface {
	RunAction
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

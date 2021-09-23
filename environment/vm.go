package environment

// VM is virtual machine.
type VM interface {
	GuestActions
	Dependencies
	Host() HostActions
	Teardown() error
}

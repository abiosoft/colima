package vm

import (
	"github.com/abiosoft/colima/environment"
)

// VM is virtual machine.
type VM interface {
	environment.GuestActions
	environment.Dependencies
	Host() environment.HostActions
	Teardown() error
}

package vm

import "github.com/abiosoft/colima/runtime"

// VM is virtual machine.
type VM interface {
	runtime.GuestActions
	runtime.Dependencies
	Host() runtime.HostActions
	Teardown() error
}

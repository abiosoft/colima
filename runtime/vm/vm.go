package vm

import "github.com/abiosoft/colima/runtime"

// VM is virtual machine.
type VM interface {
	runtime.GuestActions
	runtime.Dependencies
	Host() runtime.HostActions
	Teardown() error
}

// ColimaRuntimeEnvVar is the environment variable for checking
// the current container runtime of the Colima VM.
const ColimaRuntimeEnvVar = "COLIMA_RUNTIME"

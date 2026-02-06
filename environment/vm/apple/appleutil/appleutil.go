package appleutil

import (
	"github.com/abiosoft/colima/config/configmanager"
)

// Name is the name of the Apple Container runtime.
const Name = "apple"

// IsAppleBackend returns true if the current profile is using the Apple Container backend.
func IsAppleBackend() bool {
	c, err := configmanager.Load()
	if err != nil {
		return false
	}
	return c.Runtime == Name
}
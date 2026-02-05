package apple

import (
	"github.com/abiosoft/colima/environment/vm/apple"
)

// socktainerPkgURL is the URL for the socktainer .pkg file.
const socktainerPkgURL = apple.SocktainerPkgURL

// Dependencies returns the dependencies required for Apple Container runtime.
// Only docker is required as an external dependency.
// The container CLI is checked as a VM dependency.
// Socktainer is installed automatically when starting the daemon.
func (a appleRuntime) Dependencies() []string {
	return []string{"docker"}
}
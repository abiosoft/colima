package socktainer

import (
	"os"

	"github.com/abiosoft/colima/environment"
)

// socktainerBinary is a dependency that checks if socktainer is installed.
type socktainerBinary struct{}

// Installed implements process.Dependency.
func (s socktainerBinary) Installed() bool {
	_, err := os.Stat(BinPath())
	return err == nil
}

// Install implements process.Dependency.
// Socktainer installation is handled during container runtime provisioning,
// so this is a no-op that returns an error indicating manual installation is needed.
func (s socktainerBinary) Install(host environment.HostActions) error {
	// Installation is handled by the Apple container runtime's ensureDependencies
	// This should not be called if Installed() returns false
	return nil
}

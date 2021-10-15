package podman

import (
	"fmt"
)

func (p podmanRuntime) setupInVM() error {
	// install in VM
	err := p.guest.Run("sudo", "apt", "-y", "install", "podman")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}
	return nil
}

func (p podmanRuntime) isInstalled() bool {
	err := p.guest.RunQuiet("command", "-v", "podman")
	return err == nil
}

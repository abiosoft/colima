package podman

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

func (p podmanRuntime) getPodmanSocket(user *user.User, rootfull bool) string {
	if rootfull {
		return "/run/podman/podman.sock"
	}
	return fmt.Sprintf("/run/user/%v/podman/podman.sock", user.Uid)
}

func (p podmanRuntime) setupSocketSymlink(socket string) error {
	log := p.Logger()
	// remove existing socket (if any)
	log.Println("sudo password may be required to set up docker socket")
	err := p.host.RunInteractive("sudo", "rm", "-rf", socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	// create new symlink
	err = p.host.RunInteractive("sudo", "ln", "-s", socketSymlink(), socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	return nil
}

func socketSymlink() string {
	return filepath.Join(config.Dir(), "podman.sock")
}

func (p podmanRuntime) isSymlinkCreated(socket string) bool {
	symlink, err := p.host.RunOutput("readlink", socket)
	if err != nil {
		return false
	}
	return symlink == socketSymlink()
}

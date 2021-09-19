package docker

import (
	"fmt"
	"github.com/abiosoft/colima/util"
)

func (d dockerRuntime) setupSocketSymlink() error {
	log := d.Logger()
	// remove existing socket (if any)
	log.Println("sudo password may be required to set up docker socket")
	err := d.host.RunInteractive("sudo", "rm", "-rf", socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	// create new symlink
	err = d.host.RunInteractive("sudo", "ln", "-s", socketSymlink(), socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	return nil
}

func (d dockerRuntime) setupInVM() error {
	// install in VM
	err := d.guest.Run("sudo", "apt", "-y", "install", "docker.io")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}

	// enable buildkit by default.
	// eventually, there should be an easy way to configure docker.
	// users may want to set other configs like registries e.t.c.

	err = d.guest.Run("sudo", "mkdir", "-p", "/etc/docker")
	if err != nil {
		return fmt.Errorf("error setting up default config: %w", err)
	}

	err = d.guest.Run("sudo", "sh", "-c", `echo '{"features":{"buildkit":true}}' > /etc/docker/daemon.json`)
	if err != nil {
		return fmt.Errorf("error enabling buildkit: %w", err)
	}

	return nil
}

func (d dockerRuntime) fixUserPermission() error {
	err := d.guest.Run("sudo", "usermod", "-aG", "docker", util.User())
	if err != nil {
		return fmt.Errorf("error fixing user permission: %w", err)
	}
	return nil
}

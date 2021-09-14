package docker

import (
	"fmt"
	"github.com/abiosoft/colima/cutil"
)

func (d Docker) setupSocketSymlink() error {
	// remove existing socket (if any)
	d.log.Println("sudo password may be required to set up docker socket")
	err := d.c.Host().Run("sudo", "rm", "-rf", dockerSocket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	// create new symlink
	err = d.c.Host().Run("sudo", "ln", "-s", dockerSocketSymlink(), dockerSocket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	return nil
}

func (d Docker) setupInVM() error {
	// install in VM
	err := d.c.Guest().Run("sudo", "apt", "-y", "install", "docker.io")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}

	// enable buildkit by default.
	// eventually, there should be an easy way to configure Docker.
	// users may want to set other configs like registries e.t.c.
	err = d.c.Guest().Run("sudo", "mkdir", "-p", "/etc/docker")
	if err != nil {
		return fmt.Errorf("error setting up default config: %w", err)
	}

	err = d.c.Guest().Run("sudo", "sh", "-c", `echo '{"features":{"buildkit":true}}' > /etc/docker/daemon.json`)
	if err != nil {
		return fmt.Errorf("error enabling buildkit: %w", err)
	}

	return nil
}

func (d Docker) fixUserPermission() error {
	err := d.c.Guest().Run("sudo", "usermod", "-aG", "docker", cutil.User())
	if err != nil {
		return fmt.Errorf("error fixing user permission: %w", err)
	}
	return nil
}

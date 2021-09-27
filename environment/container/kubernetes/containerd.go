package kubernetes

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"path/filepath"
)

func installContainerdDeps(guest environment.GuestActions, r *cli.ActiveCommandChain) {
	//fix cni permission
	r.Add(func() error {
		user, err := guest.User()
		if err != nil {
			return fmt.Errorf("error retrieving username: %w", err)
		}
		if err := guest.Run("sudo", "mkdir", "-p", "/etc/cni"); err != nil {
			return err
		}
		return guest.Run("sudo", "chown", "-R", user+":"+user, "/etc/cni")
	})
	// fix cni path
	r.Add(func() error {
		cniDir := "/opt/cni/bin"
		if err := guest.Run("ls", cniDir); err == nil {
			return nil
		}

		if err := guest.Run("sudo", "mkdir", "-p", filepath.Dir(cniDir)); err != nil {
			return err
		}
		return guest.Run("sudo", "ln", "-s", "/var/lib/rancher/k3s/data/current/bin", cniDir)
	})
}

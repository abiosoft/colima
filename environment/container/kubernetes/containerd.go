package kubernetes

import (
	_ "embed"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"path/filepath"
	"strconv"
)

func installContainerdDeps(guest environment.GuestActions, a *cli.ActiveCommandChain) {
	// fix cni path
	a.Add(func() error {
		cniDir := "/usr/libexec/cni"
		if err := guest.RunQuiet("sudo", "ls", "-l", cniDir); err == nil {
			return nil
		}

		if err := guest.Run("sudo", "mkdir", "-p", filepath.Dir(cniDir)); err != nil {
			return err
		}
		return guest.Run("sudo", "ln", "-s", "/var/lib/rancher/k3s/data/current/bin", cniDir)
	})

	// fix cni config
	a.Add(func() error {
		return guest.Run("sudo", "mkdir", "-p", "/etc/cni/net.d")
	})
	a.Add(func() error {
		return guest.Run("sudo", "sh", "-c", "echo "+strconv.Quote(k3sFlannelConflist)+" > /etc/cni/net.d/10-flannel.conflist")
	})
}

//go:embed k3s-flannel.json
var k3sFlannelConflist string

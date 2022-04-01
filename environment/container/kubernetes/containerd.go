package kubernetes

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

func installContainerdDeps(guest environment.GuestActions, a *cli.ActiveCommandChain) {
	// fix missing cni plugins that are bundled with k3s
	missingBinaries := []string{"flannel"}
	a.Add(func() error {
		cniDir := "/usr/libexec/cni"
		k3sBinDir := "/var/lib/rancher/k3s/data/current/bin"
		for _, bin := range missingBinaries {
			binDest := filepath.Join(cniDir, bin)
			if err := guest.RunQuiet("sudo", "ls", "-l", binDest); err == nil {
				continue
			}

			binSource := filepath.Join(k3sBinDir, bin)
			if err := guest.Run("sudo", "ln", "-s", binSource, binDest); err != nil {
				return fmt.Errorf("error setting up cni plugin '%s': %w", bin, err)
			}
		}
		return nil
	})

	// fix cni config
	a.Add(func() error {
		flannelFile := "/etc/cni/net.d/10-flannel.conflist"
		cniConfDir := filepath.Dir(flannelFile)

		if err := guest.Run("sudo", "mkdir", "-p", cniConfDir); err != nil {
			return fmt.Errorf("error creating cni config dir: %w", err)
		}

		return guest.Write(flannelFile, k3sFlannelConflist)
	})
}

//go:embed k3s-flannel.json
var k3sFlannelConflist string

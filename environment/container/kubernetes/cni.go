package kubernetes

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
)

func installCniConfig(guest environment.GuestActions, a *cli.ActiveCommandChain) {
	// fix cni config
	a.Add(func() error {
		flannelFile := "/etc/cni/net.d/10-flannel.conflist"
		cniConfDir := filepath.Dir(flannelFile)
		if err := guest.Run("sudo", "mkdir", "-p", cniConfDir); err != nil {
			return fmt.Errorf("error creating cni config dir: %w", err)
		}

		flannel, err := embedded.Read("k3s/flannel.json")
		if err != nil {
			return fmt.Errorf("error reading embedded flannel config: %w", err)
		}
		return guest.Write(flannelFile, flannel)
	})
}

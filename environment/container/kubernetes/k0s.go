package kubernetes

import (
	"fmt"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

func installK0s(
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	k0sVersion string,
) {
	installK0sBinary(guest, a, k0sVersion)
	installK0sCluster(guest, a)
}

func installK0sBinary(
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
	k0sVersion string,
) {
	a.Add(func() error {
		k0senv := fmt.Sprintf("K0S_VERSION=%s", k0sVersion)
		cmd := fmt.Sprintf("curl --tlsv1.2 -sSf https://get.k0s.sh | sudo %s sh", k0senv)
		if err := guest.Run("sh", "-c", cmd); err != nil {
			return fmt.Errorf("failed to install k0s %w", err)
		}
		return nil
	})
}

func installK0sCluster(
	guest environment.GuestActions,
	a *cli.ActiveCommandChain,
) {
	// Initialize k0s with default configuration
	a.Add(func() error {
		return guest.Run("sudo", "k0s", "install", "controller", "--single")
	})
}

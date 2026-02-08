package incus

import (
	"fmt"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	log "github.com/sirupsen/logrus"
)

const BridgeSubnet = "192.168.100.0/24"
const bridgeGateway = "192.168.100.1/24"

// addContainerRoute adds a macOS route for the Incus container subnet
// via the VM's col0 IP address, making containers directly reachable from the host.
func (c *incusRuntime) addContainerRoute() error {
	if !util.MacOS() {
		return nil
	}

	vmIP := limautil.IPAddress(config.CurrentProfile().ID)
	if vmIP == "127.0.0.1" || vmIP == "" {
		return nil
	}

	if !util.SubnetAvailable(BridgeSubnet) {
		log.Warnf("subnet %s conflicts with host network, skipping route setup", BridgeSubnet)
		return nil
	}

	if err := embedded.InstallSudoers(c.host); err != nil {
		return fmt.Errorf("error setting up sudoers for route: %w", err)
	}

	// delete any stale route first (ignore errors)
	_ = c.removeContainerRoute()

	if err := c.host.RunQuiet("sudo", "/sbin/route", "add", "-net", BridgeSubnet, vmIP); err != nil {
		return fmt.Errorf("error adding route for %s via %s: %w", BridgeSubnet, vmIP, err)
	}

	return nil
}

// removeContainerRoute removes the macOS route for the Incus container subnet.
func (c *incusRuntime) removeContainerRoute() error {
	if !util.MacOS() {
		return nil
	}

	if !util.RouteExists(BridgeSubnet) {
		return nil
	}

	return c.host.RunQuiet("sudo", "/sbin/route", "delete", "-net", BridgeSubnet)
}

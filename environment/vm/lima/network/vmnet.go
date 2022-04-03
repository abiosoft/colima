package network

import (
	"fmt"
	"os"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

type vmnetManager struct {
	host environment.HostActions
}

func (v vmnetManager) init() error {
	// dependencies for network
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return fmt.Errorf("error preparing vmnet: %w", err)
	}

	return nil
}

func (v vmnetManager) Start() error {
	_ = v.Stop() // this is safe, nothing is done when not running

	// dependencies for network
	if err := v.init(); err != nil {
		return fmt.Errorf("error preparing network: %w", err)
	}

	return v.host.Run("sudo", colimaVmnetBinary, "start", config.Profile().ShortName)
}

func (v vmnetManager) Stop() error {
	return v.host.Run("sudo", colimaVmnetBinary, "stop", config.Profile().ShortName)
}

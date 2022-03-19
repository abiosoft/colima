package network

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

var requiredInstalls = []rootfulFile{
	sudoerFile{},
	vmnetFile{},
	colimaVmnetFile{},
}

// NetworkManager handles networking between the host and the vm.
type NetworkManager interface {
	DependenciesInstalled() bool
	InstallDependencies() error
	Start() error
	Stop() error
}

// NewManager creates a new network manager.
func NewManager(host environment.HostActions) NetworkManager {
	return &limaNetworkManager{
		launchd:   launchdManager{host},
		installer: rootfulInstaller{host},
	}
}

var _ NetworkManager = (*limaNetworkManager)(nil)

type limaNetworkManager struct {
	launchd   launchdManager
	installer rootfulInstaller
}

func (l limaNetworkManager) DependenciesInstalled() bool {
	for _, f := range requiredInstalls {
		if !l.installer.Installed(f) {
			return false
		}
	}
	return true
}

func (l limaNetworkManager) InstallDependencies() error {
	for _, f := range requiredInstalls {
		if l.installer.Installed(f) {
			continue
		}

		if err := l.installer.Install(f); err != nil {
			return err
		}
	}
	return nil
}

func (l limaNetworkManager) Start() error {
	if l.launchd.Running() {
		l.launchd.Kill()
	}
	return l.launchd.Start()
}
func (l limaNetworkManager) Stop() error {
	if l.launchd.Running() {
		if err := l.launchd.Kill(); err != nil {
			return err
		}
	}
	return l.launchd.Delete()
}

const vmnetFileName = "vmnet"

// PTPFile returns path to the ptp socket file.
func PTPFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return dir, err
	}

	return filepath.Join(dir, vmnetFileName+".ptp"), nil
}

// Dir is the network configuration directory.
func Dir() (string, error) {
	dir := filepath.Join(config.Dir(), "network")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating network directory: %w", err)
	}
	return dir, nil
}

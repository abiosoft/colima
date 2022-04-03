package network

import (
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

const colimaVmnetBinary = "/opt/colima/bin/colima-vmnet"
const VmnetGateway = "192.168.106.1"
const VmnetDHCPEnd = "192.168.106.254"
const VmnetIface = "col0"

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
	Running() (bool, error)
}

// NewManager creates a new network manager.
func NewManager(host environment.HostActions) NetworkManager {
	return &limaNetworkManager{
		host:      host,
		installer: rootfulInstaller{host: host},
		vmnet:     vmnetManager{host: host},
	}
}

var _ NetworkManager = (*limaNetworkManager)(nil)

type limaNetworkManager struct {
	host      environment.HostActions
	vmnet     vmnetManager
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
	return l.vmnet.Start()
}
func (l limaNetworkManager) Stop() error {
	return l.vmnet.Stop()
}

func (l limaNetworkManager) Running() (bool, error) {
	// validate that the vmnet socket and pid are created
	ptpFile := PTPFile()
	ptpSocket := strings.TrimSuffix(ptpFile, ".ptp") + ".pid"
	if _, err := l.host.Stat(ptpFile); err != nil {
		return false, err
	}
	if _, err := l.host.Stat(ptpSocket); err != nil {
		return false, err
	}
	return true, nil
}

const vmnetFileName = "vmnet"

// PTPFile returns path to the ptp socket file.
func PTPFile() string { return filepath.Join(Dir(), vmnetFileName+".ptp") }

// Dir is the network configuration directory.
func Dir() string { return filepath.Join(config.Dir(), "network") }

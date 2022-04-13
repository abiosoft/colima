package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

const VmnetGateway = "192.168.106.1"
const VmnetDHCPEnd = "192.168.106.254"
const VmnetIface = "col0"

var requiredInstalls = []rootfulFile{
	sudoerFile{},
	vmnetFile{},
	vmnetRunDir{},
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
	// validate that the daemon pid and vmnet ptp socket are created
	info := Info()
	if _, err := l.host.Stat(info.Vmnet.PTPFile); err != nil {
		return false, err
	}
	if _, err := l.host.Stat(info.PidFile); err != nil {
		return false, err
	}

	// check if process is actually running
	p, err := os.ReadFile(info.PidFile)
	if err != nil {
		return false, fmt.Errorf("error reading pid file: %w", err)
	}
	pid, _ := strconv.Atoi(string(p))
	if pid == 0 {
		return false, fmt.Errorf("invalid pid: %v", string(p))
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("process not found: %v", err)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, fmt.Errorf("process signal(0) returned error: %w", err)
	}

	return true, nil
}

// Dir is the network configuration directory.
func Dir() string { return filepath.Join(config.Dir(), "network") }

// RunDir is the directory to the daemon run related files. e.g. ptp, pid files
func RunDir() string { return filepath.Join(optDir, "run") }

// DaemonInfo returns the information about the network daemon.
type DaemonInfo struct {
	PidFile string
	LogFile string
	Vmnet   struct {
		PTPFile string
		PidFile string
	}
}

func Info() (d DaemonInfo) {
	d.Vmnet.PTPFile = filepath.Join(Dir(), "vmnet.ptp")
	d.Vmnet.PidFile = filepath.Join(RunDir(), "vmnet-"+config.Profile().ShortName+".pid")
	d.PidFile = filepath.Join(Dir(), "daemon.pid")
	d.LogFile = filepath.Join(Dir(), "daemon.log")
	return d
}

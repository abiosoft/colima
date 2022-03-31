package network

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
)

type launchdManager struct {
	host environment.HostActions
}

func (l launchdManager) Dir() string {
	home := util.HomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func (l launchdManager) Label() string {
	return packageNamePrefix + "." + config.Profile().ID
}

func (l launchdManager) File() string {
	return filepath.Join(l.Dir(), l.Label()+".plist")
}

func (l launchdManager) Running() bool {
	return l.host.RunQuiet("launchctl", "list", l.Label()) == nil
}

func (l launchdManager) Start() error {
	if err := l.createVmnetScript(); err != nil {
		return err
	}
	return l.host.RunQuiet("launchctl", "load", l.File())
}

func (l launchdManager) Kill() error {
	return l.host.RunQuiet("launchctl", "unload", l.File())
}

func (l launchdManager) Delete() error {
	return l.host.RunQuiet("rm", "-rf", l.File())
}

const packageNamePrefix = "com.abiosoft.colima"
const colimaVmnetBinary = "/opt/colima/bin/colima-vmnet"
const VmnetGateway = "192.168.106.1"

func (l launchdManager) createVmnetScript() error {
	if err := os.MkdirAll(l.Dir(), 0755); err != nil {
		return fmt.Errorf("error creating launchd directory: %w", err)
	}

	if stat, err := os.Stat(l.File()); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("launchd file: directory not expected at '%s'", l.File())
		}
	}

	vmnetDir, err := Dir()
	if err != nil {
		return fmt.Errorf("error starting network: %w", err)
	}

	var values = struct {
		Label   string
		Profile string
		Binary  string
		Stderr  string
		Stdout  string
		PidFile string
	}{
		Label:   l.Label(),
		Profile: config.Profile().ShortName,
		Binary:  colimaVmnetBinary,
		Stdout:  filepath.Join(vmnetDir, "vmnet.stdout"),
		Stderr:  filepath.Join(vmnetDir, "vmnet.stderr"),
		PidFile: filepath.Join(vmnetDir, "vmnet.pid"),
	}

	plist, err := embedded.ReadString("network/vmnet.plist")
	if err != nil {
		return fmt.Errorf("error preparing launchd file: %w", err)
	}
	if err := util.WriteTemplate(plist, l.File(), values); err != nil {
		return fmt.Errorf("error writing launchd file: %w", err)
	}

	return nil
}

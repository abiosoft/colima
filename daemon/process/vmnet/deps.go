package vmnet

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
)

var _ process.Dependency = sudoerFile{}

type sudoerFile struct{}

// Installed implements Dependency
func (s sudoerFile) Installed() bool { return embedded.SudoersInstalled() }

// Install implements Dependency
func (s sudoerFile) Install(host environment.HostActions) error {
	return embedded.InstallSudoers(host)
}

var _ process.Dependency = vmnetFile{}

const BinaryPath = "/opt/colima/bin/socket_vmnet"
const ClientBinaryPath = "/opt/colima/bin/socket_vmnet_client"

type vmnetFile struct{}

// Installed implements Dependency
func (v vmnetFile) Installed() bool {
	for _, bin := range v.bins() {
		if _, err := os.Stat(bin); err != nil {
			return false
		}
	}
	return true
}

func (v vmnetFile) bins() []string {
	return []string{BinaryPath, ClientBinaryPath}
}
func (v vmnetFile) Install(host environment.HostActions) error {
	arch := "x86_64"
	if runtime.GOARCH != "amd64" {
		arch = "arm64"
	}

	// read the embedded file
	gz, err := embedded.Read("network/vmnet_" + arch + ".tar.gz")
	if err != nil {
		return fmt.Errorf("error retrieving embedded vmnet file: %w", err)
	}

	// write tar to tmp directory
	f, err := os.CreateTemp("", "vmnet.tar.gz")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	if _, err := f.Write(gz); err != nil {
		return fmt.Errorf("error writing temp file: %w", err)
	}
	_ = f.Close() // not a fatal error

	defer func() {
		_ = os.Remove(f.Name())
	}()

	// extract tar to desired location
	dir := optDir
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing colima privileged dir: %w", err)
	}
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf("cd %s && tar xfz %s 2>/dev/null", dir, f.Name())); err != nil {
		return fmt.Errorf("error extracting vmnet archive: %w", err)
	}

	return nil
}

var _ process.Dependency = vmnetRunDir{}

type vmnetRunDir struct{}

// Install implements Dependency
func (v vmnetRunDir) Install(host environment.HostActions) error {
	return host.RunInteractive("sudo", "mkdir", "-p", runDir())
}

// Installed implements Dependency
func (v vmnetRunDir) Installed() bool {
	stat, err := os.Stat(runDir())
	return err == nil && stat.IsDir()
}

const optDir = "/opt/colima"

// runDir is the directory to the rootful daemon run related files. e.g. pid files
func runDir() string { return filepath.Join(optDir, "run") }

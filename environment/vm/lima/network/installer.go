package network

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
)

type rootfulInstaller struct{ host environment.HostActions }

func (r rootfulInstaller) Installed(file rootfulFile) bool { return file.Installed() }
func (r rootfulInstaller) Install(file rootfulFile) error  { return file.Install(r.host) }

type rootfulFile interface {
	Install(host environment.HostActions) error
	Installed() bool
}

var _ rootfulFile = sudoerFile{}

type sudoerFile struct{}

// Installed implements rootfulFile
func (s sudoerFile) Installed() bool {
	if _, err := os.Stat(s.path()); err != nil {
		return false
	}
	b, err := os.ReadFile(s.path())
	if err != nil {
		return false
	}
	txt, err := embedded.Read(s.embeddedPath())
	if err != nil {
		return false
	}
	return bytes.Contains(b, txt)
}

func (s sudoerFile) path() string         { return "/etc/sudoers.d/colima" }
func (s sudoerFile) embeddedPath() string { return "network/sudo.txt" }
func (s sudoerFile) Install(host environment.HostActions) error {
	// read embedded file contents
	txt, err := embedded.ReadString("network/sudo.txt")
	if err != nil {
		return fmt.Errorf("error retrieving embedded sudo file: %w", err)
	}
	// ensure parent directory exists
	dir := filepath.Dir(s.path())
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing sudoers directory: %w", err)
	}
	// persist file to desired location
	stdin := strings.NewReader(txt)
	stdout := &bytes.Buffer{}
	if err := host.RunWith(stdin, stdout, "sudo", "sh", "-c", "cat > "+s.path()); err != nil {
		return fmt.Errorf("error writing sudoers file, stderr: %s, err: %w", stdout.String(), err)
	}
	return nil
}

var _ rootfulFile = vmnetFile{}

const VmnetBinary = "/opt/colima/bin/vde_vmnet"
const vmnetLibrary = "/opt/colima/lib/libvdeplug.3.dylib"

type vmnetFile struct{}

// Installed implements rootfulFile
func (v vmnetFile) Installed() bool {
	for _, bin := range v.bins() {
		if _, err := os.Stat(bin); err != nil {
			return false
		}
	}
	return true
}

func (s vmnetFile) bins() []string {
	return []string{VmnetBinary, vmnetLibrary}
}
func (s vmnetFile) Install(host environment.HostActions) error {
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
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf("cd %s && tar xfz %s", dir, f.Name())); err != nil {
		return fmt.Errorf("error extracting vmnet archive: %w", err)
	}

	return nil
}

var _ rootfulFile = vmnetRunDir{}

type vmnetRunDir struct{}

// Install implements rootfulFile
func (v vmnetRunDir) Install(host environment.HostActions) error {
	return host.RunInteractive("sudo", "mkdir", "-p", RunDir())
}

// Installed implements rootfulFile
func (v vmnetRunDir) Installed() bool {
	stat, err := os.Stat(RunDir())
	return err == nil && stat.IsDir()
}

const optDir = "/opt/colima"

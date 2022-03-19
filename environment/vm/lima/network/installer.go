package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
)

type rootfulInstaller struct{ host environment.HostActions }

func (r rootfulInstaller) Installed(file rootfulFile) bool {
	for _, f := range file.Paths() {
		stat, err := os.Stat(f)
		if err != nil {
			return false
		}
		if file.Executable() {
			if stat.Mode()&0100 == 0 {
				return false
			}
		}
	}
	return true
}
func (r rootfulInstaller) Install(file rootfulFile) error { return file.Install(r.host) }

type rootfulFile interface {
	Paths() []string
	Executable() bool
	Install(host environment.HostActions) error
}

var _ rootfulFile = sudoerFile{}

type sudoerFile struct{}

func (s sudoerFile) Paths() []string  { return []string{"/etc/sudoers.d/colima"} }
func (s sudoerFile) Executable() bool { return false }
func (s sudoerFile) Install(host environment.HostActions) error {
	// read embedded file contents
	txt, err := embedded.ReadString("network/sudo.txt")
	if err != nil {
		return fmt.Errorf("error retrieving embedded sudo file: %w", err)
	}
	// ensure parent directory exists
	path := s.Paths()[0]
	dir := filepath.Dir(path)
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing sudoers directory: %w", err)
	}
	// persist file to desired location
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf(`echo "%s" > %s`, txt, path)); err != nil {
		return fmt.Errorf("error writing sudoers file: %w", err)
	}
	return nil
}

var _ rootfulFile = vmnetFile{}

const VmnetBinary = "/opt/colima/bin/vde_vmnet"
const vmnetLibrary = "/opt/colima/lib/libvdeplug.3.dylib"

type vmnetFile struct{}

func (s vmnetFile) Paths() []string {
	return []string{VmnetBinary, vmnetLibrary}
}
func (s vmnetFile) Executable() bool { return true }
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

	// extract tar to desired location
	dir := "/opt/colima"
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing colima privileged dir: %w", err)
	}
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf("cd %s && tar xfz %s", dir, f.Name())); err != nil {
		return fmt.Errorf("error extracting vmnet archive: %w", err)
	}
	return nil
}

var _ rootfulFile = colimaVmnetFile{}

type colimaVmnetFile struct{}

func (s colimaVmnetFile) Paths() []string  { return []string{"/opt/colima/bin/colima-vmnet"} }
func (s colimaVmnetFile) Executable() bool { return true }
func (s colimaVmnetFile) Install(host environment.HostActions) error {
	arg0, _ := exec.LookPath(os.Args[0])
	if arg0 == "" { // should never happen
		arg0 = os.Args[0]
	}
	if err := host.RunInteractive("sudo", "ln", "-sfn", arg0, s.Paths()[0]); err != nil {
		return fmt.Errorf("error creating colima-vmnet binary: %w", err)
	}
	return nil
}

package embedded

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

const sudoersPath = "/etc/sudoers.d/colima"
const sudoersEmbeddedPath = "network/sudo.txt"

// SudoersInstaller provides the ability to run commands on the host
// for installing the sudoers file.
type SudoersInstaller interface {
	RunInteractive(args ...string) error
	RunWith(stdin io.Reader, stdout io.Writer, args ...string) error
}

// SudoersInstalled checks if the sudoers file contains the expected embedded content.
func SudoersInstalled() bool {
	txt, err := Read(sudoersEmbeddedPath)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(sudoersPath)
	if err != nil {
		return false
	}
	return bytes.Contains(b, txt)
}

// InstallSudoers installs the embedded sudoers file if it is not already
// installed with the expected content. This may prompt for a sudo password.
func InstallSudoers(host SudoersInstaller) error {
	if SudoersInstalled() {
		return nil
	}

	txt, err := ReadString(sudoersEmbeddedPath)
	if err != nil {
		return fmt.Errorf("error reading embedded sudoers file: %w", err)
	}

	log.Println("setting up network permissions, sudo password may be required")

	dir := filepath.Dir(sudoersPath)
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing sudoers directory: %w", err)
	}

	stdin := strings.NewReader(txt)
	stdout := &bytes.Buffer{}
	if err := host.RunWith(stdin, stdout, "sudo", "sh", "-c", "cat > "+sudoersPath); err != nil {
		return fmt.Errorf("error writing sudoers file: %w", err)
	}

	return nil
}

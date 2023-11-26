package deb

import (
	"fmt"
	"strings"

	"github.com/abiosoft/colima/environment"
)

var manticPackages = []string{
	// docker
	"iptables",
	// k8s
	"socat",
}

var _ URISource = (*Mantic)(nil)

// Mantic is the URISource for Ubuntu Mantic packages.
type Mantic struct {
	Guest guestActions
}

// PreInstall implements URISource.
func (*Mantic) PreInstall() error {
	return nil
}

// Packages implements URISource.
func (*Mantic) Packages() []string {
	return manticPackages
}

// Name implements URISource.
func (*Mantic) Name() string {
	return "mantic-debs"
}

// URIs implements URISource.
func (m *Mantic) URIs(_ environment.Arch) ([]string, error) {
	_ = m.Guest.RunQuiet("sudo apt update -y")

	output := ""
	for _, p := range manticPackages {
		line := fmt.Sprintf(`sudo apt-get install --reinstall --no-install-recommends --print-uris -qq "%s" | cut -d"'" -f2`, p)
		out, err := m.Guest.RunOutput("sh", "-c", line)
		if err != nil {
			return nil, fmt.Errorf("error fetching dependencies list: %w", err)
		}
		output += out + " "
	}

	return strings.Fields(output), nil
}

// Install implements URISource.
func (m *Mantic) Install() error {
	return m.Guest.Run("sh", "-c", "sudo apt update && sudo apt install -f -y "+strings.Join(manticPackages, " "))
}

// Installed implements URISource.
func (m *Mantic) Installed() bool {
	args := append([]string{"dpkg", "-s"}, manticPackages...)
	return m.Guest.RunQuiet(args...) == nil
}

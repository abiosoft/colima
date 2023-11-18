package deb

import (
	"fmt"
	"strings"
)

var manticPackages = []string{
	// docker and k8s
	"docker.io", "socat",
	// utilities
	"htop", "vim", "inetutils-ping", "dnsutils",
}

// Mantic is the URISource for Ubuntu Mantic packages.
type Mantic struct {
	Guest guestActions
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
func (m *Mantic) URIs() ([]string, error) {
	output := ""
	for _, p := range manticPackages {
		line := fmt.Sprintf(`sudo apt-get install --reinstall --print-uris -qq "%s" | cut -d"'" -f2`, p)
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
	return m.Guest.Run("sh", "-c", "sudo apt install -y "+strings.Join(manticPackages, " "))
}

var _ URISource = (*Mantic)(nil)

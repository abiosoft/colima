package deb

import (
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/environment"
)

var dockerPackages = []string{
	"docker-ce",
	"docker-ce-cli",
	"containerd.io",
	"docker-buildx-plugin",
	"docker-compose-plugin",
}

var _ URISource = (*Docker)(nil)

// Docker is the URISource for Docker CE packages.
type Docker struct {
	Host  hostActions
	Guest guestActions
}

// PreInstall implements URISource.
func (d *Docker) PreInstall() error {
	return d.Guest.RunQuiet("sh", "-c", "sudo apt purge -y docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc")
}

// Install implements URISource.
func (d *Docker) Install() error {
	args := []string{"--channel", dockerChannel()}
	if v := os.Getenv("DOCKER_INSTALL_VERSION"); v != "" {
		args = append(args, "--version", v)
	}
	return d.Guest.Run("sh", "-c",
		`curl -fsSL https://get.docker.com -o /tmp/get-docker.sh && sudo sh /tmp/get-docker.sh `+strings.Join(args, " "),
	)
}

// Name implements URISource.
func (*Docker) Name() string {
	return "docker-ce"
}

// Packages implements URISource.
func (*Docker) Packages() []string {
	return dockerPackages
}

// URIs implements URISource.
func (d *Docker) URIs(arch environment.Arch) ([]string, error) {
	var uris []string

	pkgFiles, err := d.pkgFiles(arch)
	if err != nil {
		return nil, fmt.Errorf("error getting package names and version: %w", err)
	}

	for _, file := range pkgFiles {
		uri := d.debPackageBaseURI(arch) + file
		uris = append(uris, uri)
	}

	return uris, nil
}

func (d Docker) pkgFiles(arch environment.Arch) ([]string, error) {
	script := fmt.Sprintf(`curl -sL https://download.docker.com/linux/ubuntu/dists/mantic/%s/binary-%s/Packages | grep '^Filename: ' | awk -F'/' '{print $NF}'`, dockerChannel(), arch.Value().GoArch())
	filenames, err := d.Host.RunOutput("sh", "-c", script)
	if err != nil {
		return nil, fmt.Errorf("error retrieving deb package filenames: %w", err)
	}

	return strings.Fields(filenames), nil
}

func (d Docker) debPackageBaseURI(arch environment.Arch) string {
	return fmt.Sprintf("https://download.docker.com/linux/ubuntu/dists/mantic/pool/stable/%s/", arch.GoArch())
}

func dockerChannel() string {
	if v := os.Getenv("DOCKER_INSTALL_CHANNEL"); v != "" {
		return v
	}
	return "stable"
}

// Installed implements URISource.
func (d Docker) Installed() bool {
	return d.Guest.RunQuiet("sh", "-c", `command -v dockerd && systemctl show -p FragmentPath docker | grep docker.service`) == nil
}

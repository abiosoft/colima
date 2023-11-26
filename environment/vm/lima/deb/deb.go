package deb

import (
	"github.com/abiosoft/colima/environment"
)

type (
	hostActions  = environment.HostActions
	guestActions = environment.GuestActions
)

// URISource is the source for fetching URI for deb packages.
type URISource interface {
	// Name is the name for the URISource.
	Name() string
	// Packages is the list of package names.
	Packages() []string
	// URIs return the list of URIs to download the deb files.
	URIs(arch environment.Arch) ([]string, error)
	// PreInstall is done before the deb package are installed.
	PreInstall() error
	// Install installs the packages directly using the internet.
	Install() error
	// Installed returns if the deb packages are already installed
	Installed() bool
}

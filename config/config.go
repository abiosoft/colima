package config

import (
	"net"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/util"
)

const (
	AppName = "colima"
)

var profile = ProfileInfo{ID: AppName, DisplayName: AppName, ShortName: "default"}

// SetProfile sets the profile name for the application.
// This is an avenue to test Colima without breaking an existing stable setup.
// Not perfect, but good enough for testing.
func SetProfile(profileName string) {
	profile = Profile(profileName)
}

// Profile converts string to profile info.
func Profile(name string) ProfileInfo {
	var i ProfileInfo

	switch name {
	case "", AppName, "default":
		i.ID = AppName
		i.DisplayName = AppName
		i.ShortName = "default"
		return i
	}

	// sanitize
	name = strings.TrimPrefix(name, "colima-")

	// if custom profile is specified,
	// use a prefix to prevent possible name clashes
	i.ID = "colima-" + name
	i.DisplayName = "colima [profile=" + name + "]"
	i.ShortName = name
	return i
}

// CurrentProfile returns the current application profile.
func CurrentProfile() ProfileInfo { return profile }

// ProfileInfo is information about the colima profile.
type ProfileInfo struct {
	ID          string
	DisplayName string
	ShortName   string
}

// VersionInfo is the application version info.
type VersionInfo struct {
	Version  string
	Revision string
}

func AppVersion() VersionInfo { return VersionInfo{Version: appVersion, Revision: revision} }

var (
	appVersion = "development"
	revision   = "unknown"
)

// Config is the application config.
type Config struct {
	CPU          int               `yaml:"cpu,omitempty"`
	Disk         int               `yaml:"disk,omitempty"`
	Memory       int               `yaml:"memory,omitempty"`
	Arch         string            `yaml:"arch,omitempty"`
	CPUType      string            `yaml:"cpuType,omitempty"`
	ForwardAgent bool              `yaml:"forwardAgent,omitempty"`
	Network      Network           `yaml:"network,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"` // environment variables
	Hostname     string            `yaml:"hostname"`

	// VM
	VMType    string `yaml:"vmType,omitempty"`
	VZRosetta bool   `yaml:"rosetta,omitempty"`

	// volume mounts
	Mounts       []Mount `yaml:"mounts,omitempty"`
	MountType    string  `yaml:"mountType,omitempty"`
	MountINotify bool    `yaml:"mountInotify,omitempty"`

	// Runtime is one of docker, containerd.
	Runtime         string `yaml:"runtime,omitempty"`
	ActivateRuntime *bool  `yaml:"autoActivate,omitempty"`

	// Kubernetes configuration
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`

	// Docker configuration
	Docker map[string]any `yaml:"docker,omitempty"`

	// provision scripts
	Provision []Provision `yaml:"provision,omitempty"`

	// SSH config generation
	SSHConfig bool `yaml:"sshConfig,omitempty"`
}

// Kubernetes is kubernetes configuration
type Kubernetes struct {
	Enabled bool     `yaml:"enabled"`
	Version string   `yaml:"version"`
	K3sArgs []string `yaml:"k3sArgs"`
}

// Network is VM network configuration
type Network struct {
	Address      bool              `yaml:"address"`
	DNSResolvers []net.IP          `yaml:"dns"`
	DNSHosts     map[string]string `yaml:"dnsHosts"`
}

// Mount is volume mount
type Mount struct {
	Location   string `yaml:"location"`
	MountPoint string `yaml:"mountPoint,omitempty"`
	Writable   bool   `yaml:"writable"`
}

type Provision struct {
	Mode   string `yaml:"mode"`
	Script string `yaml:"script"`
}

func (c Config) MountsOrDefault() []Mount {
	if len(c.Mounts) > 0 {
		return c.Mounts
	}

	return []Mount{
		{Location: util.HomeDir(), Writable: true},
		{Location: filepath.Join("/tmp", CurrentProfile().ID), Writable: true},
	}
}

// AutoActivate returns if auto-activation of host client config is enabled.
func (c Config) AutoActivate() bool {
	if c.ActivateRuntime == nil {
		return true
	}
	return *c.ActivateRuntime
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

// CtxKey returns the context key for config.
func CtxKey() any {
	return struct{ name string }{name: "colima_config"}
}

func (c Config) DriverLabel() string {
	if util.MacOS13OrNewer() && c.VMType == "vz" {
		return "macOS Virtualization.Framework"
	}
	return "QEMU"
}

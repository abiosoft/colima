package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/util"
)

const AppName = "colima"
const SubprocessProfileEnvVar = "COLIMA_PROFILE"

var profile = ProfileInfo{ID: AppName, DisplayName: AppName, ShortName: "default"}

// SetProfile sets the profile name for the application.
// This is an avenue to test Colima without breaking an existing stable setup.
// Not perfect, but good enough for testing.
func SetProfile(profileName string) {
	switch profileName {
	case "", AppName, "default":
		return
	}

	// if custom profile is specified,
	// use a prefix to prevent possible name clashes
	profile.ID = "colima-" + profileName
	profile.DisplayName = "colima [profile=" + profileName + "]"
	profile.ShortName = profileName
}

// Profile returns the current application profile.
func Profile() ProfileInfo { return profile }

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
	DNS          []net.IP          `yaml:"dns,omitempty"` // DNS nameservers
	Env          map[string]string `yaml:"env,omitempty"` // environment variables

	// volume mounts
	Mounts    []Mount `yaml:"mounts,omitempty"`
	MountType string  `yaml:"mountType,omitempty"`

	// Runtime is one of docker, containerd.
	Runtime string `yaml:"runtime,omitempty"`

	// Kubernetes configuration
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`

	// Docker configuration
	Docker map[string]any `yaml:"docker,omitempty"`

	// layer
	Ubuntu bool `yaml:"ubuntuLayer,omitempty"`
}

// Kubernetes is kubernetes configuration
type Kubernetes struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version"`
	Ingress bool   `yaml:"ingress"`
}

const (
	UserModeDriver = "slirp"
	VmnetDriver    = "vmnet"
	GVProxyDriver  = "gvproxy"
)

// Network is VM network configuration
type Network struct {
	Address bool   `yaml:"address"`
	Driver  string `yaml:"driver"`
}

// Mount is volume mount
type Mount struct {
	Location string `yaml:"location"`
	Writable bool   `yaml:"writable"`
}

// CleanPath returns the absolute path to the mount location.
func (m Mount) CleanPath() (string, error) {
	split := strings.SplitN(m.Location, ":", 2)
	str := os.ExpandEnv(split[0])

	if strings.HasPrefix(str, "~") {
		str = strings.Replace(str, "~", util.HomeDir(), 1)
	}

	str = filepath.Clean(str)
	if !filepath.IsAbs(str) {
		return "", fmt.Errorf("relative paths not supported for mount '%s'", m.Location)
	}

	return strings.TrimSuffix(str, "/") + "/", nil
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

// CtxKey returns the context key for config.
func CtxKey() any {
	return struct{ name string }{name: "colima_config"}
}

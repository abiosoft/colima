package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

const AppName = "colima"

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

// requiredDir is a directory that must exist on the filesystem
type requiredDir struct {
	once sync.Once
	// dir is a func to enable deferring the value of the directory
	// until execution time.
	// if dir() returns an error, a fatal error is triggered.
	dir func() (string, error)
}

// Dir returns the directory path.
// It ensures the directory is created on the filesystem by calling
// `mkdir` prior to returning the directory path.
func (r *requiredDir) Dir() string {
	dir, err := r.dir()
	if err != nil {
		logrus.Fatal(fmt.Errorf("cannot fetch required directory: %w", err))
	}

	r.once.Do(func() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logrus.Fatal(fmt.Errorf("cannot make required directory: %w", err))
		}
	})

	return dir
}

var (
	configDir requiredDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, ".colima", profile.ShortName), nil
		},
	}

	cacheDir requiredDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "colima"), nil
		},
	}
)

// AppDir returns the application directory.
func AppDir() string { return filepath.Dir(configDir.Dir()) }

// Dir returns the configuration directory.
func Dir() string { return configDir.Dir() }

// File returns the config file.
func File() string { return configFile() }

// CacheDir returns the cache directory.
func CacheDir() string { return cacheDir.Dir() }

const configFileName = "colima.yaml"

func configFile() string { return filepath.Join(configDir.Dir(), configFileName) }

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
	Mounts     []Mount  `yaml:"mounts,omitempty"`
	MountsFlag []string `yaml:"-"`
	MountType  string   `yaml:"mountType,omitempty"`

	// Runtime is one of docker, containerd.
	Runtime string `yaml:"runtime,omitempty"`

	// Kubernetes configuration
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`

	// Docker configuration
	Docker map[string]any `yaml:"docker,omitempty"`
}

// Kubernetes is kubernetes configuration
type Kubernetes struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version"`
	Ingress bool   `yaml:"ingress"`
}

// Network is VM network configuration
type Network struct {
	Address  bool `yaml:"address"`
	UserMode bool `yaml:"userMode"`
}

// Mount is volume mount
type Mount struct {
	Location string `yaml:"location"`
	Writable bool   `yaml:"writable"`
}

// CleanPath returns the absolute path to the mount location.
func (m Mount) CleanPath() (string, error) {
	split := strings.SplitN(string(m.Location), ":", 2)
	str := os.ExpandEnv(split[0])

	if strings.HasPrefix(str, "~") {
		str = strings.Replace(str, "~", util.HomeDir(), 1)
	}

	str = filepath.Clean(str)
	if !filepath.IsAbs(str) {
		return "", fmt.Errorf("relative paths not supported for mount '%s'", string(m.Location))
	}

	return strings.TrimSuffix(str, "/") + "/", nil
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

// CtxKey returns the context key for config.
func CtxKey() any {
	return struct{ name string }{name: "colima_config"}
}

package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/util/yamlutil"
	"gopkg.in/yaml.v3"
)

const AppName = "colima"

var profile = ProfileInfo{ID: AppName, DisplayName: AppName}

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
}

// Profile returns the current application profile.
func Profile() ProfileInfo { ensureInit(); return profile }

// ProfileInfo is information about the colima profile.
type ProfileInfo struct {
	ID          string
	DisplayName string
}

// VersionInfo is the application version info.
type VersionInfo struct {
	Version  string
	Revision string
}

func AppVersion() VersionInfo {
	ensureInit()
	return VersionInfo{Version: appVersion, Revision: revision}
}

var (
	appVersion = "development"
	revision   = "unknown"

	configDir string
	cacheDir  string
)

// Dir returns the configuration directory.
func Dir() string { ensureInit(); return configDir }

// CacheDir returns the cache directory.
func CacheDir() string { ensureInit(); return cacheDir }

var initDone = false

func ensureInit() {
	if initDone {
		return
	}

	{
		// prepare config directory
		dir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(fmt.Errorf("cannot fetch user config directory: %w", err))
		}
		configDir = filepath.Join(dir, "."+profile.ID)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			log.Fatal(fmt.Errorf("cannot create config directory: %w", err))
		}
	}

	{
		// prepare cache directory
		dir, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(fmt.Errorf("cannot fetch user config directory: %w", err))
		}
		cacheDir = filepath.Join(dir, profile.ID)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			log.Fatal(fmt.Errorf("cannot create cache directory: %w", err))
		}
	}
	initDone = true
}

const configFileName = "colima.yaml"

func configFile() string { ensureInit(); return filepath.Join(configDir, configFileName) }

// Save saves the config.
func Save(c Config) error {
	return yamlutil.WriteYAML(c, configFile())
}

// Load loads the config.
// Error is only returned if the config file exists but could not be loaded.
// No error is returned if the config file does not exist.
func Load() (Config, error) {
	if _, err := os.Stat(configFile()); err != nil {
		// config file does not exist
		return Config{}, nil
	}
	var c Config
	b, err := os.ReadFile(configFile())
	if err != nil {
		return c, fmt.Errorf("could not load previous settings: %w", err)
	}

	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return c, fmt.Errorf("could not load previous settings: %w", err)
	}
	return c, nil
}

// Teardown deletes the config.
func Teardown() error {
	if _, err := os.Stat(configDir); err == nil {
		return os.RemoveAll(configDir)
	}
	return nil
}

// Config is the application config.
type Config struct {
	// Virtual Machine
	VM VM `yaml:"vm"`

	// Runtime is one of docker, containerd, podman.
	Runtime string `yaml:"runtime"`

	// Kubernetes sets if kubernetes should be enabled.
	Kubernetes Kubernetes `yaml:"kubernetes"`

	// Network address to forward VM ports to.
	PortInterface net.IP
}

// Kubernetes is kubernetes configuration
type Kubernetes struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version"`
}

// VM is virtual machine configuration.
type VM struct {
	CPU    int    `yaml:"cpu"`
	Disk   int    `yaml:"disk"`
	Memory int    `yaml:"memory"`
	Arch   string `yaml:"arch"`

	// auto generated
	SSHPort int `yaml:"-"`

	// volume mounts
	Mounts []string `yaml:"mounts"`

	// do not persist. i.e. discarded on VM shutdown
	DNS []net.IP          `yaml:"-"` // DNS nameservers
	Env map[string]string `yaml:"-"` // environment variables
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

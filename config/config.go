package config

import (
	"fmt"
	"github.com/abiosoft/colima/util"
	"gopkg.in/yaml.v3"
	"log"
	"net"
	"os"
	"path/filepath"
)

const appName = "colima"

func AppName() string    { return appName }
func AppVersion() string { return appVersion }

var (
	appVersion = "v0.2.0-devel"

	configDir string
	cacheDir  string

	// TODO change config location
	sshPort = 41122
)

// SSHPort returns the SSH port for the VM
// TODO change location
func SSHPort() int { return sshPort }

// Dir returns the configuration directory.
func Dir() string { return configDir }

// LogFile returns the path the command log output.
func LogFile() string {
	return filepath.Join(cacheDir, "out.log")
}

func init() {
	{
		// prepare config directory
		dir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(fmt.Errorf("cannot fetch user config directory: %w", err))
		}
		configDir = filepath.Join(dir, "."+appName)
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
		cacheDir = filepath.Join(dir, appName)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			log.Fatal(fmt.Errorf("cannot create cache directory: %w", err))
		}
	}
}

const configFileName = "colima.yaml"

func configFile() string {
	return filepath.Join(configDir, configFileName)
}

// Save saves the config.
func Save(c Config) error {
	return util.WriteYAML(c, configFile())
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

	// Runtime is one of docker, containerd.
	Runtime string `yaml:"runtime"`

	// Kubernetes sets if kubernetes should be enabled.
	Kubernetes bool `yaml:"kubernetes"`
}

// VM is virtual machine configuration.
type VM struct {
	CPU    int `yaml:"cpu"`
	Disk   int `yaml:"disk"`
	Memory int `yaml:"memory"`

	// do not persist. i.e. discarded on VM shutdown
	DNS []net.IP          `yaml:"-"` // DNS nameservers
	Env map[string]string `yaml:"-"` // environment variables

	// internal use
	SSHPort int `yaml:"-"`
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

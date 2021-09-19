package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
)

const appName = "colima"

func AppName() string    { return appName }
func AppVersion() string { return appVersion }

var (
	appVersion = "devel"

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
	CPU    int               `yaml:"cpu"`
	Disk   int               `yaml:"disk"`
	Memory int               `yaml:"memory"`
	DNS    []net.IP          `yaml:"dns"` // DNS nameservers
	Env    map[string]string `yaml:"env"` // environment variables

	// internal use
	SSHPort int `yaml:"-"`
}

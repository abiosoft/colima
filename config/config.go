package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const appName = "colima"

func AppName() string    { return appName }
func AppVersion() string { return appVersion }

var (
	appVersion = "devel"

	configDir string

	// TODO change config location
	sshPort = 41122
)

// SSHPort returns the SSH port for the VM
// TODO change location
func SSHPort() int { return sshPort }

// Dir returns the configuration directory.
func Dir() string {
	return filepath.Join(configDir, appName)
}

func init() {
	// prepare config directory
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(fmt.Errorf("cannot fetch user config directory: %w", err))
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(fmt.Errorf("cannot create config directory: %w", err))
	}
	configDir = dir
}

// Config is the application config.
type Config struct {
	// VM is virtual machine configuration.
	VM struct {
		CPU    int      `yaml:"cpu"`
		Disk   int      `yaml:"disk"`
		Memory int      `yaml:"memory"`
		DNS    []string `yaml:"dns"` // DNS nameservers
		Env    []string `yaml:"env"` // environment variables
	} `yaml:"vm"`

	// Runtime is one of docker, containerd.
	Runtime string `yaml:"runtime"`

	// Kubernetes sets if kubernetes should be enabled.
	Kubernetes bool `yaml:"kubernetes"`

	// true when user changes config with flag
	Changed bool `yaml:"-"`
}

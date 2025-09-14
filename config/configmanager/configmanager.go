package configmanager

import (
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/yamlutil"
	"gopkg.in/yaml.v3"
)

// Save saves the config.
func Save(c config.Config) error {
	return yamlutil.Save(c, config.CurrentProfile().File())
}

// SaveFromFile loads configuration from file and save as config.
func SaveFromFile(file string) error {
	c, err := LoadFrom(file)
	if err != nil {
		return err
	}
	return Save(c)
}

// SaveToFile saves configuration to file.
func SaveToFile(c config.Config, file string) error {
	return yamlutil.Save(c, file)
}

// LoadFrom loads config from file.
func LoadFrom(file string) (config.Config, error) {
	var c config.Config
	b, err := os.ReadFile(file)
	if err != nil {
		return c, fmt.Errorf("could not load config from file: %w", err)
	}

	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return c, fmt.Errorf("could not load config from file: %w", err)
	}

	return c, nil
}

// ValidateConfig validates config before we use it
func ValidateConfig(c config.Config) error {
	validMountTypes := map[string]bool{"9p": true, "sshfs": true}
	validPortForwarders := map[string]bool{"grpc": true, "ssh": true}

	if util.MacOS13OrNewer() {
		validMountTypes["virtiofs"] = true
	}
	if _, ok := validMountTypes[c.MountType]; !ok {
		return fmt.Errorf("invalid mountType: '%s'", c.MountType)
	}
	validVMTypes := map[string]bool{"qemu": true}
	if util.MacOS13OrNewer() {
		validVMTypes["vz"] = true
	}
	if _, ok := validVMTypes[c.VMType]; !ok {
		return fmt.Errorf("invalid vmType: '%s'", c.VMType)
	}
	if c.VMType == "qemu" {
		if err := util.AssertQemuImg(); err != nil {
			return fmt.Errorf("cannot use vmType: '%s', error: %w", c.VMType, err)
		}
	}

	if c.DiskImage != "" {
		if strings.HasPrefix(c.DiskImage, "http://") || strings.HasPrefix(c.DiskImage, "https://") {
			return fmt.Errorf("cannot use diskImage: remote URLs not supported, only local files can be specified")
		}
	}

	if _, ok := validPortForwarders[c.PortForwarder]; !ok {
		return fmt.Errorf("invalid port forwarder: '%s'", c.PortForwarder)
	}

	return nil
}

// Load loads the config.
// Error is only returned if the config file exists but could not be loaded.
// No error is returned if the config file does not exist.
func Load() (c config.Config, err error) {
	f := config.CurrentProfile().File()
	if _, err := os.Stat(f); err != nil {
		return c, nil
	}

	return LoadFrom(f)
}

// LoadInstance is like Load but returns the config of the currently running instance.
func LoadInstance() (config.Config, error) {
	return LoadFrom(config.CurrentProfile().StateFile())
}

// Teardown deletes the config.
func Teardown() error {
	dir := config.CurrentProfile().ConfigDir()
	if _, err := os.Stat(dir); err == nil {
		return os.RemoveAll(dir)
	}
	return nil
}

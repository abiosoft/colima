package configmanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/yamlutil"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Save saves the config.
func Save(c config.Config) error {
	return yamlutil.Save(c, config.File())
}

// SaveFromFile loads configuration from file and save as config.
func SaveFromFile(file string) error {
	cf, err := LoadFrom(file)
	if err != nil {
		return err
	}
	return Save(cf)
}

// SaveToFile saves configuration to file.
func SaveToFile(c config.Config, file string) error {
	return yamlutil.Save(c, file)
}

// oldConfigFile returns the path to config file of versions <0.4.0.
// TODO: remove later, only for backward compatibility
func oldConfigFile() string {
	_, configFileName := filepath.Split(config.File())
	return filepath.Join(os.Getenv("HOME"), "."+config.CurrentProfile().ID, configFileName)
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

	return nil
}

// Load loads the config.
// Error is only returned if the config file exists but could not be loaded.
// No error is returned if the config file does not exist.
func Load() (config.Config, error) {
	cFile := config.File()
	if _, err := os.Stat(cFile); err != nil {
		oldCFile := oldConfigFile()

		// config file does not exist, check older version for backward compatibility
		if _, err := os.Stat(oldCFile); err != nil {
			return config.Config{}, nil
		}

		// older version exists
		logrus.Infof("settings from older %s version detected and copied", config.AppName)
		if err := cli.Command("cp", oldCFile, cFile).Run(); err != nil {
			logrus.Warn(fmt.Errorf("error copying config: %w, proceeding with defaults", err))
			return config.Config{}, nil
		}
	}

	return LoadFrom(cFile)
}

// Teardown deletes the config.
func Teardown() error {
	if _, err := os.Stat(config.Dir()); err == nil {
		return os.RemoveAll(config.Dir())
	}
	return nil
}

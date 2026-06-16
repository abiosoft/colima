package configmanager

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
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
	validPortForwarders := map[string]bool{"grpc": true, "ssh": true, "none": true}

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
	if util.MacOS13OrNewerOnArm() {
		validVMTypes["krunkit"] = true
	}
	if c.VMType == "krunkit" && !util.MacOS13OrNewerOnArm() {
		return fmt.Errorf("vmType 'krunkit' is only available on macOS with Apple Silicon")
	}
	if _, ok := validVMTypes[c.VMType]; !ok {
		return fmt.Errorf("invalid vmType: '%s'", c.VMType)
	}
	if c.VMType == "qemu" {
		if err := util.AssertQemuImg(); err != nil {
			return fmt.Errorf("cannot use vmType: '%s', error: %w", c.VMType, err)
		}
	}
	if c.VMType == "krunkit" {
		if err := util.AssertKrunkit(); err != nil {
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

	if c.Network.GatewayAddress != nil {
		if err := validateGatewayAddress(c.Network.GatewayAddress); err != nil {
			return err
		}
	}

	if err := validatePhysicalDisks(c.PhysicalDisks); err != nil {
		return err
	}

	return nil
}

func validatePhysicalDisks(disks []config.PhysicalDisk) error {
	if len(disks) == 0 {
		return nil
	}
	if !util.MacOS() {
		return fmt.Errorf("physicalDisks is currently only supported on macOS")
	}

	validName := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	validDiskDevice := regexp.MustCompile(`^disk[0-9]+s[0-9]+$`)
	validRawDiskDevice := regexp.MustCompile(`^rdisk[0-9]+s[0-9]+$`)
	seenNames := map[string]bool{}
	seenDevices := map[string]string{}
	seenGuestMounts := map[string]string{}
	seenHostMounts := map[string]string{}

	for _, disk := range disks {
		if disk.Name == "" {
			return fmt.Errorf("physicalDisks entries require a name")
		}
		if !validName.MatchString(disk.Name) {
			return fmt.Errorf("invalid physicalDisks name %q: use letters, numbers, dot, underscore, or dash", disk.Name)
		}
		if seenNames[disk.Name] {
			return fmt.Errorf("duplicate physicalDisks name %q", disk.Name)
		}
		seenNames[disk.Name] = true

		if disk.Device == "" {
			return fmt.Errorf("physicalDisks.%s requires device", disk.Name)
		}
		if !filepath.IsAbs(disk.Device) || !strings.HasPrefix(disk.Device, "/dev/disk") {
			return fmt.Errorf("physicalDisks.%s device must be an absolute /dev/diskNsM path", disk.Name)
		}
		if !validDiskDevice.MatchString(filepath.Base(disk.Device)) {
			return fmt.Errorf("physicalDisks.%s must target a partition, not a whole disk", disk.Name)
		}
		if other := seenDevices[disk.Device]; other != "" {
			return fmt.Errorf("physicalDisks.%s uses the same device as physicalDisks.%s", disk.Name, other)
		}
		seenDevices[disk.Device] = disk.Name

		if disk.RawDevice != "" {
			if !filepath.IsAbs(disk.RawDevice) || !validRawDiskDevice.MatchString(filepath.Base(disk.RawDevice)) {
				return fmt.Errorf("physicalDisks.%s rawDevice must be an absolute /dev/rdiskNsM path", disk.Name)
			}
		}

		switch disk.Backend {
		case "", "auto", "nbd":
		default:
			return fmt.Errorf("physicalDisks.%s backend %q is unsupported", disk.Name, disk.Backend)
		}

		switch disk.FSType {
		case "", "auto", "ext4", "xfs", "btrfs":
		default:
			return fmt.Errorf("physicalDisks.%s fsType %q is unsupported", disk.Name, disk.FSType)
		}

		if disk.MountPoint != "" {
			if !filepath.IsAbs(disk.MountPoint) {
				return fmt.Errorf("physicalDisks.%s mountPoint must be absolute", disk.Name)
			}
			if other := seenGuestMounts[disk.MountPoint]; other != "" {
				return fmt.Errorf("physicalDisks.%s uses the same mountPoint as physicalDisks.%s", disk.Name, other)
			}
			seenGuestMounts[disk.MountPoint] = disk.Name
		}

		if disk.HostAccess.Enabled {
			switch disk.HostAccess.Driver {
			case "", "nfs":
			default:
				return fmt.Errorf("physicalDisks.%s hostAccess.driver %q is unsupported", disk.Name, disk.HostAccess.Driver)
			}
			if disk.HostAccess.MountPoint != "" {
				if !filepath.IsAbs(disk.HostAccess.MountPoint) {
					return fmt.Errorf("physicalDisks.%s hostAccess.mountPoint must be absolute", disk.Name)
				}
				if other := seenHostMounts[disk.HostAccess.MountPoint]; other != "" {
					return fmt.Errorf("physicalDisks.%s uses the same hostAccess.mountPoint as physicalDisks.%s", disk.Name, other)
				}
				seenHostMounts[disk.HostAccess.MountPoint] = disk.Name
			}
		}
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

// Validates that gateway is a valid IPv4 address and that the last octet is “2”.
// Lima uses the last octet as 2 for gateways.
func validateGatewayAddress(gateway net.IP) error {
	ip4 := gateway.To4()
	if ip4 == nil {
		return fmt.Errorf("gateway %q is not IPv4", gateway)
	}

	// Check last octet
	if ip4[3] != 2 {
		return fmt.Errorf("the last octet of gateway %q is not 2", gateway)
	}

	return nil
}

package limautil

import (
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

const colimaDiffDiskFile = "diffdisk"

// ColimaDiffDisk returns path to the diffdisk for the colima VM.
func ColimaDiffDisk(profileID string) string {
	return filepath.Join(config.ProfileFromName(profileID).LimaInstanceDir(), colimaDiffDiskFile)
}

const networkFile = "networks.yaml"

// NetworkFile returns path to the network file.
func NetworkFile() string {
	return filepath.Join(config.LimaDir(), "_config", networkFile)
}

// NetworkAssetsDirecotry returns the directory for the generated network assets.
func NetworkAssetsDirectory() string {
	return filepath.Join(config.LimaDir(), "_networks")
}

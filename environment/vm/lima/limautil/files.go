package limautil

import (
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

const colimaStateFile = "colima.yaml"

// ColimaStateFile returns path to the colima state yaml file.
func ColimaStateFile(profileID string) string {
	return filepath.Join(LimaHome(), config.Profile(profileID).ID, colimaStateFile)
}

const colimaDiffDiskFile = "diffdisk"

// ColimaDiffDisk returns path to the diffdisk for the colima VM.
func ColimaDiffDisk(profileID string) string {
	return filepath.Join(LimaHome(), config.Profile(profileID).ID, colimaDiffDiskFile)
}

const networkFile = "networks.yaml"

// NetworkFile returns path to the network file.
func NetworkFile() string {
	return filepath.Join(LimaHome(), "_config", networkFile)
}

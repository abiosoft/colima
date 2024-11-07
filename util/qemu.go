package util

import (
	"fmt"
	"os/exec"
)

// AssertQemuImg checks if qemu-img is available.
func AssertQemuImg() error {
	cmd := "qemu-img"
	if _, err := exec.LookPath(cmd); err != nil {
		return fmt.Errorf("%s not found, run 'brew install %s' to install", cmd, "qemu")
	}

	return nil
}

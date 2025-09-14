package limautil

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/abiosoft/colima/config"
)

// HasDisk checks if a lima disk exists for the current instance.
func HasDisk() bool {
	name := config.CurrentProfile().ID

	var resp struct {
		Name string `json:"name"`
	}

	cmd := Limactl("disk", "list", "--json", name)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return false
	}

	if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
		return false
	}

	return resp.Name == name
}

// CreateDisk creates a lima disk with size in GiB.
func CreateDisk(size int) error {
	name := config.CurrentProfile().ID

	var buf bytes.Buffer
	cmd := Limactl("disk", "create", name, "--size", fmt.Sprintf("%dGiB", size))
	cmd.Stderr = &buf
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error creating lima disk: %w, output: %s", err, buf.String())
	}

	return nil
}

// ResizeDisk resizes disk to new size
func ResizeDisk(size int) error {
	name := config.CurrentProfile().ID

	var buf bytes.Buffer
	cmd := Limactl("disk", "resize", name, "--size", fmt.Sprintf("%dGiB", size))
	cmd.Stderr = &buf
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error resizing disk: %w, output: %s", err, buf.String())
	}

	return nil
}

// DeleteDisk deletes lima disk for the current instance.
func DeleteDisk() error {
	name := config.CurrentProfile().ID

	var buf bytes.Buffer
	cmd := Limactl("disk", "delete", name)
	cmd.Stderr = &buf
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error deleting lima disk: %w, output: %s", err, buf.String())
	}

	return nil
}

// MountPoint returns the lima disk mount point for the current instance.
func MountPoint() string { return fmt.Sprintf("/mnt/lima-%s", config.CurrentProfile().ID) }

package native

import (
	"fmt"
	"os"
	"path/filepath"
)

// Read reads a file directly from the host filesystem.
// Lima's file.go uses SSH cat to read files inside the VM.
func (n nativeVM) Read(fileName string) (string, error) {
	b, err := os.ReadFile(fileName)
	if err != nil {
		return "", fmt.Errorf("cannot read file '%s': %w", fileName, err)
	}
	return string(b), nil
}

// Write writes a file directly to the host filesystem.
// Lima's file.go uses SSH to pipe content into the VM.
func (n nativeVM) Write(fileName string, body []byte) error {
	dir := filepath.Dir(fileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory '%s': %w", dir, err)
	}
	return os.WriteFile(fileName, body, 0644)
}

// Stat returns file info directly from the host filesystem.
func (n nativeVM) Stat(fileName string) (os.FileInfo, error) {
	return os.Stat(fileName)
}

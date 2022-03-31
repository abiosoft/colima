package util

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"runtime"
)

// HomeDir returns the user home directory.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// this should never happen
		log.Fatal(fmt.Errorf("error retrieving home directory: %w", err))
	}
	return home
}

// SHA256Hash computes a sha256sum of a string.
func SHA256Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// MacOS returns if the current OS is macOS.
func MacOS() bool {
	return runtime.GOOS == "darwin"
}

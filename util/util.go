package util

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
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

type SHA256 [32]byte

func (s SHA256) String() string { return fmt.Sprintf("%x", s[:]) }

// SHA256Hash computes a sha256sum of a string.
func SHA256Hash(s string) SHA256 {
	return sha256.Sum256([]byte(s))
}

// MacOS returns if the current OS is macOS.
func MacOS() bool {
	return runtime.GOOS == "darwin"
}

// AppendToPath appends directory to PATH.
func AppendToPath(path, dir string) string {
	if path == "" {
		return dir
	}
	if dir == "" {
		return path
	}
	return dir + ":" + path
}

// RemoveFromPath removes directory from PATH.
func RemoveFromPath(path, dir string) string {
	var envPath []string
	for _, p := range strings.Split(path, ":") {
		if strings.TrimSuffix(p, "/") == strings.TrimSuffix(dir, "/") || strings.TrimSpace(p) == "" {
			continue
		}
		envPath = append(envPath, p)
	}
	return strings.Join(envPath, ":")
}

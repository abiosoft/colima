package util

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
)

// HomeDir returns the user home directory.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// this should never happen
		logrus.Fatal(fmt.Errorf("error retrieving home directory: %w", err))
	}
	return home
}

// MacOS returns if the current OS is macOS.
func MacOS() bool {
	return runtime.GOOS == "darwin"
}

// MacOS13OrNewer returns if the current OS is macOS 13 or newer.
func MacOS13OrNewerOnM1() bool {
	return runtime.GOARCH == "arm64" && MacOS13OrNewer()
}

// MacOS13OrNewer returns if the current OS is macOS 13 or newer.
func MacOS13OrNewer() bool {
	if !MacOS() {
		return false
	}
	ver, err := macOSProductVersion()
	if err != nil {
		logrus.Warnln(fmt.Errorf("error retrieving macOS version: %w", err))
		return false
	}

	cver, err := semver.NewVersion("13.0.0")
	if err != nil {
		logrus.Warnln(fmt.Errorf("error parsing version: %w", err))
		return false
	}

	return cver.Compare(*ver) <= 0
}

// RosettaRunning checks if Rosetta process is running.
func RosettaRunning() bool {
	if !MacOS() {
		return false
	}
	cmd := cli.Command("pgrep", "oahd")
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
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

// RandomAvailablePort returns an available port on the host machine.
func RandomAvailablePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		logrus.Fatal(fmt.Errorf("error picking an available port: %w", err))
	}

	if err := listener.Close(); err != nil {
		logrus.Fatal(fmt.Errorf("error closing temporary port listener: %w", err))
	}

	return listener.Addr().(*net.TCPAddr).Port
}

// ShellSplit splits cmd into arguments using.
func ShellSplit(cmd string) []string {
	split, err := shlex.Split(cmd)
	if err != nil {
		logrus.Warnln("error splitting into args: %w", err)
		logrus.Warnln("falling back to whitespace split", err)
		split = strings.Fields(cmd)
	}

	return split
}

// CleanPath returns the absolute path to the mount location.
// If location is an empty string, nothing is done.
func CleanPath(location string) (string, error) {
	if location == "" {
		return "", nil
	}

	str := os.ExpandEnv(location)

	if strings.HasPrefix(str, "~") {
		str = strings.Replace(str, "~", HomeDir(), 1)
	}

	str = filepath.Clean(str)
	if !filepath.IsAbs(str) {
		return "", fmt.Errorf("relative paths not supported for mount '%s'", location)
	}

	return strings.TrimSuffix(str, "/") + "/", nil
}

// macOSProductVersion returns the host's macOS version.
func macOSProductVersion() (*semver.Version, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	// output is like "12.3.1\n"
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %v: %w", cmd.Args, err)
	}
	verTrimmed := strings.TrimSpace(string(b))
	// macOS 12.4 returns just "12.4\n"
	for strings.Count(verTrimmed, ".") < 2 {
		verTrimmed += ".0"
	}
	verSem, err := semver.NewVersion(verTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse macOS version %q: %w", verTrimmed, err)
	}
	return verSem, nil
}

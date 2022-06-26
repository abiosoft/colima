package util

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

// ShellSplit splids cmd into arguments using.
func ShellSplit(cmd string) []string {
	split, err := shlex.Split(cmd)
	if err != nil {
		logrus.Warnln("error splitting into args: %w", err)
		logrus.Warnln("falling back to whitespace split", err)
		split = strings.Fields(cmd)
	}

	return split
}

const EnvColimaBinary = "COLIMA_BINARY"

// Executable returns the path name for the executable that started
// the current process.
func Executable() string {
	e, err := func(s string) (string, error) {
		// prioritize env var in case this is a nested process
		if e := os.Getenv(EnvColimaBinary); e != "" {
			return e, nil
		}

		if filepath.IsAbs(s) {
			return s, nil
		}

		e, err := exec.LookPath(s)
		if err != nil {
			return "", fmt.Errorf("error looking up '%s' in PATH: %w", s, err)
		}

		abs, err := filepath.Abs(e)
		if err != nil {
			return "", fmt.Errorf("error computing absolute path of '%s': %w", e, err)
		}

		return abs, nil
	}(os.Args[0])

	if err != nil {
		// this should never happen, thereby it is safe to do
		logrus.Warnln(fmt.Errorf("cannot detect current running executable: %w", err))
		logrus.Warnln("falling back to first CLI argument")
		return os.Args[0]
	}

	return e
}

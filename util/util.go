package util

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
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

// Executable returns the path name for the executable that started
// the current process. There is no guarantee that the path is still
// pointing to the correct executable. If a symlink was used to start
// the process, depending on the operating system, the result might
// be the symlink or the path it pointed to. If a stable result is
// needed, path/filepath.EvalSymlinks might help.
//
// Executable returns an absolute path unless an error occurred.
//
// The main use case is finding resources located relative to an
// executable.
func Executable() string {
	e, err := os.Executable()
	if err != nil {
		// this should never happen, thereby it is safe to do
		logrus.Warnln(fmt.Errorf("cannot detect current running executable: %w, falling back to first CLI argument", err))
		return os.Args[0]
	}
	return e
}

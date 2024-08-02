package util

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
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

// HostIPAddresses returns all IPv4 addresses on the host.
func HostIPAddresses() []net.IP {
	var addresses []net.IP
	ints, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for i := range ints {
		split := strings.Split(ints[i].String(), "/")
		addr := net.ParseIP(split[0]).To4()
		// ignore default loopback
		if addr != nil && addr.String() != "127.0.0.1" {
			addresses = append(addresses, addr)
		}
	}

	return addresses
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

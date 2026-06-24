package util

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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

// isPortAvailable checks if a specific port is available on the host.
func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	if err := listener.Close(); err != nil {
		return false
	}
	return true
}

// FindAvailablePort finds the first available port starting from startPort.
// It checks up to maxAttempts consecutive ports (startPort, startPort+1, ...).
// Returns the available port and true if found, or 0 and false if no port is available.
func FindAvailablePort(startPort, maxAttempts int) (int, bool) {
	for i := range maxAttempts {
		port := startPort + i
		if isPortAvailable(port) {
			return port, true
		}
	}
	return 0, false
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

// SubnetAvailable checks if a subnet (in CIDR notation) does not conflict
// with any existing host network interface addresses.
func SubnetAvailable(subnet string) bool {
	_, cidr, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip = ip.To4(); ip == nil {
			continue
		}
		if cidr.Contains(ip) {
			return false
		}
	}

	return true
}

// RouteExists checks if a route exists for the given subnet on macOS.
func RouteExists(subnet string) bool {
	if !MacOS() {
		return false
	}

	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return false
	}

	// macOS netstat shows /24 subnets without trailing .0
	// e.g. "192.168.100" instead of "192.168.100.0"
	networkAddr := strings.TrimSuffix(ip.String(), ".0")

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && (fields[0] == networkAddr || fields[0] == subnet) {
			return true
		}
	}

	return false
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

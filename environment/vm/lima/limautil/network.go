package limautil

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// network interface for shared network in the virtual machine.
const NetInterface = "col0"

// network metric for the route
const NetMetric uint32 = 300
const NetMetricPreferred uint32 = 100

// IPAddress returns the ip address for profile.
// It returns the PTP address if networking is enabled or falls back to 127.0.0.1.
// It is guaranteed to return a value.
//
// TODO: unnecessary round-trip is done to get instance details from Lima.
func IPAddress(profileID string) string {
	const fallback = "127.0.0.1"
	instance, err := getInstance(profileID)
	if err != nil {
		return fallback
	}

	if len(instance.Network) > 0 {
		for _, n := range instance.Network {
			if n.Interface == NetInterface {
				return getIPAddress(profileID, n.Interface)
			}
		}
	}

	return fallback
}

// InternalIPAddress returns the internal IP address for the profile.
func InternalIPAddress(profileID string) string {
	return getIPAddress(profileID, "eth0")
}

func getIPAddress(profileID, interfaceName string) string {
	var buf bytes.Buffer
	// TODO: this should be less hacky
	cmd := Limactl("shell", profileID, "sh", "-c",
		`ip -4 addr show `+interfaceName+` | grep inet | awk -F' ' '{print $2 }' | cut -d/ -f1`)
	cmd.Stderr = nil
	cmd.Stdout = &buf

	_ = cmd.Run()
	return strings.TrimSpace(buf.String())
}

type LimaNetworkConfig struct {
	Mode    string `yaml:"mode"`
	Gateway string `yaml:"gateway"`
	Netmask string `yaml:"netmask"`
}

type LimaNetwork struct {
	Networks struct {
		UserV2 LimaNetworkConfig `yaml:"user-v2"`
	} `yaml:"networks"`
}

// AdjustGateway ensures gateway is a valid IPv4 address and then ensures the last octet is “2”.
// If it’s valid IPv4 but last octet != 2, it changes the last octet to 2.
func AdjustGateway(gateway string) (string, error) {
	ip := net.ParseIP(gateway)
	if ip == nil {
		return "", fmt.Errorf("gateway %q is not a valid IP address", gateway)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return "", fmt.Errorf("gateway %q is not IPv4", gateway)
	}

	parts := strings.Split(gateway, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("gateway %q does not have 4 octets", gateway)
	}

	if parts[3] != "2" {
		parts[3] = "2"
		gateway = strings.Join(parts, ".")
	}

	return gateway, nil
}

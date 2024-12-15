package limautil

import (
	"bytes"
	"strings"
)

// network interface for shared network in the virtual machine.
const NetInterface = "col0"

// network metric for the route
const NetMetric = 300

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

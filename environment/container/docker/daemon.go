package docker

import (
	"encoding/json"
	"fmt"
	"net"
)

const daemonFile = "/etc/docker/daemon.json"

func (d dockerRuntime) createDaemonFile(conf map[string]any) error {
	if conf == nil {
		conf = map[string]any{}
	}

	// enable buildkit (if not set by user)
	if _, ok := conf["features"]; !ok {
		conf["features"] = map[string]any{"buildkit": true}
	}

	// enable cgroupfs for k3s (if not set by user)
	if _, ok := conf["exec-opts"]; !ok {
		conf["exec-opts"] = []string{"native.cgroupdriver=cgroupfs"}
	} else if opts, ok := conf["exec-opts"].([]string); ok {
		conf["exec-opts"] = append(opts, "native.cgroupdriver=cgroupfs")
	}

	// get host-gateway ip from the guest
	ip, err := d.guest.RunOutput("sh", "-c", "host host.lima.internal | awk -F' ' '{print $NF}'")
	if err != nil {
		return fmt.Errorf("error retrieving host gateway IP address: %w", err)
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid host gateway IP address: %s", ip)
	}

	// set host-gateway ip to loopback interface (if not set by user)
	if _, ok := conf["host-gateway"]; !ok {
		conf["host-gateway-ip"] = ip
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling daemon.json: %w", err)
	}
	return d.guest.Write(daemonFile, b)
}

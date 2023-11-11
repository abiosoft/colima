package docker

import (
	"encoding/json"
	"fmt"
	"net"
)

const daemonFile = "/etc/docker/daemon.json"

func (d dockerRuntime) createDaemonFile(conf map[string]any, env map[string]string) error {
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
	ip, err := d.guest.RunOutput("sh", "-c", "grep 'host.lima.internal' /etc/hosts | awk -F' ' '{print $1}'")
	if err != nil {
		return fmt.Errorf("error retrieving host gateway IP address: %w", err)
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid host gateway IP address: '%s'", ip)
	}

	// set host-gateway ip to loopback interface (if not set by user)
	if _, ok := conf["host-gateway"]; !ok {
		conf["host-gateway-ip"] = ip
	}

	// add proxy vars if set
	// according to https://docs.docker.com/config/daemon/systemd/#httphttps-proxy
	if vars := d.proxyEnvVars(env); !vars.empty() {
		if vars.http != "" {
			conf["http-proxy"] = vars.http
		}
		if vars.https != "" {
			conf["https-proxy"] = vars.https
		}
		if vars.no != "" {
			conf["no-proxy"] = vars.https
		}
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling daemon.json: %w", err)
	}
	return d.guest.Write(daemonFile, b)
}

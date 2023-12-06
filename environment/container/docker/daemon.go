package docker

import (
	"encoding/json"
	"fmt"
	"net"
)

const daemonFile = "/etc/docker/daemon.json"
const hostGatewayIPKey = "host-gateway-ip"

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
	// remove host-gateway-ip if set by the user
	// to avoid clash with systemd configuration
	delete(conf, hostGatewayIPKey)

	// add proxy vars if set
	// according to https://docs.docker.com/config/daemon/systemd/#httphttps-proxy
	if vars := d.proxyEnvVars(env); !vars.empty() {
		proxyConf := map[string]any{}
		if vars.http != "" {
			proxyConf["http-proxy"] = vars.http
		}
		if vars.https != "" {
			proxyConf["https-proxy"] = vars.https
		}
		if vars.no != "" {
			proxyConf["no-proxy"] = vars.no
		}
		conf["proxies"] = proxyConf
	}

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling daemon.json: %w", err)
	}
	return d.guest.Write(daemonFile, b)
}

func (d dockerRuntime) addHostGateway(conf map[string]any) error {
	// get host-gateway ip from the guest
	ip, err := d.guest.RunOutput("sh", "-c", "grep 'host.lima.internal' /etc/hosts | awk -F' ' '{print $1}'")
	if err != nil {
		return fmt.Errorf("error retrieving host gateway IP address: %w", err)
	}
	// if set by the user, use the user specified value
	if _, ok := conf[hostGatewayIPKey]; ok {
		if gip, ok := conf[hostGatewayIPKey].(string); ok {
			ip = gip
		}
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid host gateway IP address: '%s'", ip)
	}

	// set host-gateway ip as systemd service file
	content := fmt.Sprintf(systemdUnitFileContent, ip)
	if err := d.guest.Write(systemdUnitFilename, []byte(content)); err != nil {
		return fmt.Errorf("error creating systemd unit file: %w", err)
	}

	return nil
}

const systemdUnitFilename = "/etc/systemd/system/docker.service.d/docker.conf"
const systemdUnitFileContent string = `
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock --host-gateway-ip=%s
`

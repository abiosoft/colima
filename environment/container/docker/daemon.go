package docker

import (
	"encoding/json"
	"fmt"
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

	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling daemon.json: %w", err)
	}
	return d.guest.Write(daemonFile, string(b))
}

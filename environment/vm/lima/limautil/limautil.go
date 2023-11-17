package limautil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/daemon/process/vmnet"
)

// EnvLimaHome is the environment variable for the Lima directory.
const EnvLimaHome = "LIMA_HOME"

// LimactlCommand is the limactl command.
const LimactlCommand = "limactl"

// Limactl prepares a limactl command.
func Limactl(args ...string) *exec.Cmd {
	cmd := cli.Command(LimactlCommand, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, EnvLimaHome+"="+LimaHome())
	return cmd
}

// Instance returns current instance.
func Instance() (InstanceInfo, error) {
	return getInstance(config.CurrentProfile().ID)
}

// InstanceConfig returns the current instance config.
func InstanceConfig() (config.Config, error) {
	i, err := Instance()
	if err != nil {
		return config.Config{}, err
	}
	return i.Config()
}

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
			if n.Interface == vmnet.NetInterface {
				return getIPAddress(profileID, n.Interface)
			}
		}
	}

	return fallback
}

// InstanceInfo is the information about a Lima instance
type InstanceInfo struct {
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Arch    string `json:"arch,omitempty"`
	CPU     int    `json:"cpus,omitempty"`
	Memory  int64  `json:"memory,omitempty"`
	Disk    int64  `json:"disk,omitempty"`
	Dir     string `json:"dir,omitempty"`
	Network []struct {
		VNL       string `json:"vnl,omitempty"`
		Interface string `json:"interface,omitempty"`
	} `json:"network,omitempty"`
	IPAddress string `json:"address,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
}

// Running checks if the instance is running.
func (i InstanceInfo) Running() bool { return i.Status == limaStatusRunning }

// Config returns the current Colima config
func (i InstanceInfo) Config() (config.Config, error) {
	return configmanager.LoadFrom(ColimaStateFile(i.Name))
}

// Lima statuses
const (
	limaStatusRunning = "Running"
)

func getInstance(profileID string) (InstanceInfo, error) {
	var i InstanceInfo
	var buf bytes.Buffer
	cmd := Limactl("list", profileID, "--json")
	cmd.Stderr = nil
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return i, fmt.Errorf("error retrieving instance: %w", err)
	}

	if buf.Len() == 0 {
		return i, fmt.Errorf("instance '%s' does not exist", config.Profile(profileID).DisplayName)
	}

	if err := json.Unmarshal(buf.Bytes(), &i); err != nil {
		return i, fmt.Errorf("error retrieving instance: %w", err)
	}
	return i, nil
}

// Instances returns Lima instances created by colima.
func Instances(ids ...string) ([]InstanceInfo, error) {
	limaIDs := make([]string, len(ids))
	for i := range ids {
		limaIDs = append(limaIDs, config.Profile(ids[i]).ID)
	}
	args := append([]string{"list", "--json"}, limaIDs...)

	var buf bytes.Buffer
	cmd := Limactl(args...)
	cmd.Stderr = nil
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error retrieving instances: %w", err)
	}

	var instances []InstanceInfo
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		var i InstanceInfo
		line := scanner.Bytes()
		if err := json.Unmarshal(line, &i); err != nil {
			return nil, fmt.Errorf("error retrieving instances: %w", err)
		}

		// limit to colima instances
		if !strings.HasPrefix(i.Name, "colima") {
			continue
		}

		if i.Running() {
			for _, n := range i.Network {
				if n.Interface == vmnet.NetInterface {
					i.IPAddress = getIPAddress(i.Name, vmnet.NetInterface)
				}
			}
			conf, _ := i.Config()
			i.Runtime = getRuntime(conf)
		}

		// rename to local friendly names
		i.Name = config.Profile(i.Name).ShortName

		// network is low level, remove
		i.Network = nil

		instances = append(instances, i)
	}

	return instances, nil
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

func getRuntime(conf config.Config) string {
	var runtime string

	switch conf.Runtime {
	case "docker", "containerd":
		runtime = conf.Runtime
	default:
		return ""
	}

	if conf.Kubernetes.Enabled {
		runtime += "+k3s"
	}
	return runtime
}

// LimaHome returns the config directory for Lima.
func LimaHome() string {
	// if LIMA_HOME env var is set, obey it.
	if dir := os.Getenv(EnvLimaHome); dir != "" {
		return dir
	}

	return config.LimaDir()
}

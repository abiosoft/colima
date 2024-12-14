package limautil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
)

// Instance returns current instance.
func Instance() (InstanceInfo, error) {
	return getInstance(config.CurrentProfile().ID)
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
	return configmanager.LoadFrom(config.ProfileFromName(i.Name).StateFile())
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
		return i, fmt.Errorf("instance '%s' does not exist", config.ProfileFromName(profileID).DisplayName)
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
		limaIDs[i] = config.ProfileFromName(ids[i]).ID
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
				if n.Interface == NetInterface {
					i.IPAddress = getIPAddress(i.Name, NetInterface)
				}
			}
			conf, _ := i.Config()
			i.Runtime = getRuntime(conf)
		}

		// rename to local friendly names
		i.Name = config.ProfileFromName(i.Name).ShortName

		// network is low level, remove
		i.Network = nil

		instances = append(instances, i)
	}

	return instances, nil
}

// RunningInstances return Lima instances that are has a running status.
func RunningInstances() ([]InstanceInfo, error) {
	allInstances, err := Instances()
	if err != nil {
		return nil, err
	}

	var runningInstances []InstanceInfo
	for _, instance := range allInstances {
		if instance.Running() {
			runningInstances = append(runningInstances, instance)
		}
	}

	return runningInstances, nil
}

func getRuntime(conf config.Config) string {
	var runtime string

	switch conf.Runtime {
	case "docker", "containerd", "incus":
		runtime = conf.Runtime
	case "none":
		return "none"
	default:
		return ""
	}

	if conf.Kubernetes.Enabled {
		runtime += "+k3s"
	}
	return runtime
}

package limautil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	if conf, err := i.Config(); err == nil {
		if conf.Disk > 0 {
			i.Disk = config.Disk(conf.Disk).Int()
		}
	}
	return i, nil
}

// limaInstances returns instances retrieved via limactl.
// Returns nil slice (not error) if limactl is unavailable.
func limaInstances(ids ...string) ([]InstanceInfo, error) {
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
		// limactl not available — not an error for native-only setups
		return nil, nil
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

		conf, _ := i.Config()
		if i.Running() {
			for _, n := range i.Network {
				if n.Interface == NetInterface {
					i.IPAddress = getIPAddress(i.Name, NetInterface)
				}
			}
			i.Runtime = getRuntime(conf)
		}

		// rename to local friendly names
		i.Name = config.ProfileFromName(i.Name).ShortName

		// network is low level, remove
		i.Network = nil

		// report correct disk usage
		if conf.Disk > 0 {
			i.Disk = config.Disk(conf.Disk).Int()
		}

		instances = append(instances, i)
	}

	return instances, nil
}

// NativeInstances scans the lima directory for native-mode colima profiles
// by reading state files directly, without requiring limactl.
func NativeInstances(ids ...string) []InstanceInfo {
	limaDir := config.LimaDir()

	entries, err := os.ReadDir(limaDir)
	if err != nil {
		return nil
	}

	// Build filter set from requested IDs
	filterSet := make(map[string]bool)
	for _, id := range ids {
		p := config.ProfileFromName(id)
		filterSet[p.ID] = true
	}

	var instances []InstanceInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Only colima profiles
		if !strings.HasPrefix(name, "colima") {
			continue
		}

		// Apply filter if specified
		if len(filterSet) > 0 && !filterSet[name] {
			continue
		}

		// Read state file
		stateFile := filepath.Join(limaDir, name, "colima.yaml")
		conf, err := configmanager.LoadFrom(stateFile)
		if err != nil {
			continue
		}

		// Only native profiles
		if conf.VMType != "native" {
			continue
		}

		profile := config.ProfileFromName(name)

		inst := InstanceInfo{
			Name:    profile.ShortName,
			Arch:    conf.Arch,
			CPU:     conf.CPU,
			Memory:  int64(conf.Memory * 1024 * 1024 * 1024), // GiB to bytes
			Runtime: getRuntime(conf),
		}

		if conf.Disk > 0 {
			inst.Disk = config.Disk(conf.Disk).Int()
		}

		// Check if native instance is running by checking systemd service
		switch conf.Runtime {
		case "docker":
			if isSystemdActive("docker.service") {
				inst.Status = limaStatusRunning
				inst.IPAddress = getNativeIPAddress()
			} else {
				inst.Status = "Stopped"
			}
		case "containerd":
			if isSystemdActive("containerd.service") {
				inst.Status = limaStatusRunning
				inst.IPAddress = getNativeIPAddress()
			} else {
				inst.Status = "Stopped"
			}
		case "incus":
			if isSystemdActive("incus.service") {
				inst.Status = limaStatusRunning
				inst.IPAddress = getNativeIPAddress()
			} else {
				inst.Status = "Stopped"
			}
		default:
			inst.Status = "Stopped"
		}

		instances = append(instances, inst)
	}

	return instances
}

// isSystemdActive checks if a systemd service is active.
func isSystemdActive(service string) bool {
	var buf bytes.Buffer
	cmd := exec.Command("systemctl", "is-active", service)
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.TrimSpace(buf.String()) == "active"
}

// getNativeIPAddress returns the host's primary IP address for native mode.
func getNativeIPAddress() string {
	var buf bytes.Buffer
	cmd := exec.Command("hostname", "-I")
	cmd.Stdout = &buf
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	fields := strings.Fields(buf.String())
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// Instances returns all Colima instances (Lima-managed + native).
func Instances(ids ...string) ([]InstanceInfo, error) {
	// Get Lima-managed instances (gracefully handles missing limactl)
	limaInsts, err := limaInstances(ids...)
	if err != nil {
		return nil, err
	}

	// Get native instances from state files
	nativeInsts := NativeInstances(ids...)

	// Merge, deduplicating by name
	seen := make(map[string]bool)
	var all []InstanceInfo

	for _, inst := range limaInsts {
		seen[inst.Name] = true
		all = append(all, inst)
	}
	for _, inst := range nativeInsts {
		if !seen[inst.Name] {
			all = append(all, inst)
		}
	}

	return all, nil
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


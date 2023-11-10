package limautil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

const (
	LayerEnvVar = "COLIMA_LAYER_SSH_PORT"
)

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

// ShowSSH runs the show-ssh command in Lima.
// returns the ssh output, if in layer, and an error if any
func ShowSSH(profileID string) (resp struct {
	Output string
	File   struct {
		Lima   string
		Colima string
	}
}, err error) {
	ssh := sshConfig(profileID)
	sshConf, err := ssh.Contents()
	if err != nil {
		return resp, fmt.Errorf("error retrieving ssh config: %w", err)
	}

	resp.Output = replaceSSHConfig(sshConf, profileID)
	resp.File.Lima = ssh.File()
	resp.File.Colima = config.SSHConfigFile()
	return resp, nil
}

func replaceSSHConfig(conf string, profileID string) string {
	profileID = config.Profile(profileID).ID

	var out bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(conf))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Host ") {
			line = "Host " + profileID
		}

		_, _ = fmt.Fprintln(&out, line)
	}
	return out.String()
}

// Lima statuses
const (
	limaStatusRunning = "Running"
)

func getInstance(profileID string) (InstanceInfo, error) {
	var i InstanceInfo
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "list", profileID, "--json")
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
	cmd := cli.Command("limactl", args...)
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
					i.IPAddress = getIPAddress(i.Name, i.Network[0].Interface)
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
	cmd := cli.Command("limactl", "shell", profileID, "sh", "-c",
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

var limaHome string

// LimaHome returns the config directory for Lima.
func LimaHome() string {
	if limaHome != "" {
		return limaHome
	}

	home, err := func() (string, error) {
		var buf bytes.Buffer
		cmd := cli.Command("limactl", "info")
		cmd.Stdout = &buf

		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("error retrieving lima info: %w", err)
		}

		var resp struct {
			LimaHome string `json:"limaHome"`
		}
		if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
			return "", fmt.Errorf("error decoding json for lima info: %w", err)
		}
		if resp.LimaHome == "" {
			return "", fmt.Errorf("error retrieving lima info, ensure lima version is >0.7.4")
		}

		return resp.LimaHome, nil
	}()

	if err != nil {
		err = fmt.Errorf("error detecting Lima config directory: %w", err)
		logrus.Warnln(err)
		logrus.Warnln("falling back to default '$HOME/.lima'")
		home = filepath.Join(util.HomeDir(), ".lima")
	}

	limaHome = home
	return home
}

const colimaStateFileName = "colima.yaml"

// ColimaStateFile returns path to the colima state yaml file.
func ColimaStateFile(profileID string) string {
	return filepath.Join(LimaHome(), config.Profile(profileID).ID, colimaStateFileName)
}

const colimaDiffDisk = "diffdisk"

// ColimaDiffDisk returns path to the diffdisk for the colima VM.
func ColimaDiffDisk(profileID string) string {
	return filepath.Join(LimaHome(), config.Profile(profileID).ID, colimaDiffDisk)
}

const sshConfigFile = "ssh.config"

// sshConfig is the ssh configuration file for a Colima profile.
type sshConfig string

// Contents returns the content of the SSH config file.
func (s sshConfig) Contents() (string, error) {
	profile := config.Profile(string(s))
	b, err := os.ReadFile(s.File())
	if err != nil {
		return "", fmt.Errorf("error retrieving Lima SSH config file for profile '%s': %w", strings.TrimPrefix(profile.DisplayName, "lima"), err)
	}
	return string(b), nil
}

// File returns the path to the SSH config file.
func (s sshConfig) File() string {
	profile := config.Profile(string(s))
	return filepath.Join(LimaHome(), profile.ID, sshConfigFile)
}

package limautil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
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
// TODO: unnecessary round-trip is done to get instance details from Lima.
func IPAddress(profileID string) string {
	// profile = toUserFriendlyName(profile)

	const fallback = "127.0.0.1"
	instance, err := getInstance(profileID)
	if err != nil {
		return fallback
	}

	if len(instance.Network) > 0 {
		return getIPAddress(profileID, instance.Network[0].Interface)
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

func (i InstanceInfo) Running() bool { return i.Status == limaStatusRunning }

func (i InstanceInfo) Config() (config.Config, error) {
	return configmanager.LoadFrom(ColimaStateFile(i.Name))
}

// ShowSSH runs the show-ssh command in Lima.
// returns the ssh output, if in layer, and an error if any
func ShowSSH(profileID string, layer bool, format string) (resp struct {
	Output    string
	IPAddress string
	Layer     bool
}, err error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "show-ssh", "--format", format, profileID)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return resp, fmt.Errorf("error retrieving ssh config: %w", err)
	}

	ip := IPAddress(profileID)
	var port int
	if layer {
		port, _ = ubuntuSSHPort(profileID)
		// if layer is active and public IP is available, use the fixed port
		if port > 0 && ip != "127.0.0.1" {
			port = 23
		}
	} else {
		ip = "127.0.0.1"
	}

	out := buf.String()
	switch format {
	case "config":
		out = replaceSSHConfig(out, profileID, ip, port)
	case "cmd", "args":
		out = replaceSSHCmd(out, profileID, ip, port)
	default:
		return resp, fmt.Errorf("unsupported format '%v'", format)
	}

	resp.Output = out
	resp.IPAddress = ip
	resp.Layer = port > 0
	return resp, nil
}

func replaceSSHCmd(cmd string, name string, ip string, port int) string {
	var out []string

	for _, s := range util.ShellSplit(cmd) {
		if port > 0 {
			if strings.HasPrefix(s, "ControlPath=") {
				s = "ControlPath=" + strconv.Quote(filepath.Join(config.Dir(), "ssh.sock"))
			}
			if strings.HasPrefix(s, "Port=") {
				s = "Port=" + strconv.Itoa(port)
			}
			if strings.HasPrefix(s, "Hostname=") {
				s = "Hostname=" + ip
			}
		}

		out = append(out, s)
	}

	if out[len(out)-1] == "lima-"+name {
		out[len(out)-1] = ip
	}

	return strings.Join(out, " ")
}
func replaceSSHConfig(conf string, name string, ip string, port int) string {
	var out bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(conf))

	hasPrefix := func(line, s string) (pad string, ok bool) {
		if s != "" && strings.HasPrefix(strings.TrimSpace(line), s) {
			return line[:strings.Index(line, s[:1])], true
		}
		return "", false
	}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Host ") {
			line = "Host " + name
		}

		if port > 0 {
			if pad, ok := hasPrefix(line, "ControlPath "); ok {
				line = pad + "ControlPath " + strconv.Quote(filepath.Join(config.Dir(), "ssh.sock"))
			}

			if pad, ok := hasPrefix(line, "Hostname "); ok {
				line = pad + "Hostname " + ip
			}

			if pad, ok := hasPrefix(line, "Port"); ok {
				line = pad + "Port " + strconv.Itoa(port)
			}
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
	if err := json.Unmarshal(buf.Bytes(), &i); err != nil {
		return i, fmt.Errorf("error retrieving instance: %w", err)
	}

	return i, nil
}

// Instances returns Lima instances created by colima.
func Instances() ([]InstanceInfo, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "list", "--json")
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
			if len(i.Network) > 0 && i.Network[0].Interface != "" {
				i.IPAddress = getIPAddress(i.Name, i.Network[0].Interface)
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
		`ifconfig `+interfaceName+` | grep "inet addr:" | awk -F' ' '{print $2}' | awk -F':' '{print $2}'`)
	cmd.Stderr = nil
	cmd.Stdout = &buf

	_ = cmd.Run()
	return strings.TrimSpace(buf.String())
}

func ubuntuSSHPort(profileID string) (int, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "shell", profileID, "--", "sh", "-c", "echo $"+LayerEnvVar)
	cmd.Stdout = &buf
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("cannot retrieve ubuntu layer SSH port: %w", err)
	}

	port, err := strconv.Atoi(strings.TrimSpace(buf.String()))
	if err != nil {
		return 0, fmt.Errorf("invalid ubuntu layer SSH port '%d': %w", port, err)
	}

	return port, nil
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
	return filepath.Join(LimaHome(), profileID, colimaStateFileName)
}

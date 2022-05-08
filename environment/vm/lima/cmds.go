package lima

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
)

// InstanceInfo is the information about a Lima instance
type InstanceInfo struct {
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Arch    string `json:"arch,omitempty"`
	CPU     int    `json:"cpus,omitempty"`
	Memory  int64  `json:"memory,omitempty"`
	Disk    int64  `json:"disk,omitempty"`
	Network []struct {
		VNL       string `json:"vnl,omitempty"`
		Interface string `json:"interface,omitempty"`
	} `json:"network,omitempty"`
	IPAddress string `json:"address,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
}

// Instances returns Lima instances created by colima.
func Instances() ([]InstanceInfo, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "list", "--json")
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

		if i.Status == "Running" {
			if len(i.Network) > 0 && i.Network[0].Interface != "" {
				i.IPAddress = getIPAddress(i.Name, i.Network[0].Interface)
			}
			i.Runtime = getRuntime(i.Name)
		}

		// rename to local friendly names
		i.Name = toUserFriendlyName(i.Name)

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
	cmd.Stdout = &buf

	_ = cmd.Run()
	return strings.TrimSpace(buf.String())
}

func UbuntuSSHPort(profileID string) (int, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "shell", profileID, "--", "sh", "-c", "echo $"+layerEnvVar)
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("cannot retrieve ubuntu layer SSH port: %w", err)
	}

	port, err := strconv.Atoi(strings.TrimSpace(buf.String()))
	if err != nil {
		return 0, fmt.Errorf("invalid ubuntu layer SSH port '%d': %w", port, err)
	}

	return port, nil
}

func getRuntime(profile string) string {
	run := func(args ...string) bool {
		cmd := "limactl"
		args = append([]string{"shell", profile}, args...)
		c := cli.Command(cmd, args...)
		c.Stdout = nil
		c.Stderr = nil
		return c.Run() == nil
	}

	var runtime string
	// docker
	if run("docker", "info") {
		runtime = "docker"
	} else if run("nerdctl", "info") {
		runtime = "containerd"
	}

	// nothing is running
	if runtime == "" {
		return ""
	}

	if run("kubectl", "cluster-info") {
		runtime += "+k3s"
	}
	return runtime
}

// IPAddress returns the ip address for profile.
// It returns the PTP address is networking is enabled or falls back to 127.0.0.1
// TODO: unnecessary round-trip is done to get instance details from Lima.
func IPAddress(profile string) string {
	profile = toUserFriendlyName(profile)

	const fallback = "127.0.0.1"
	instances, err := Instances()
	if err != nil {
		return fallback
	}

	for _, instance := range instances {
		if instance.Name == profile {
			if instance.IPAddress != "" {
				return instance.IPAddress
			}
			break
		}
	}

	return fallback
}

// ShowSSH runs the show-ssh command in Lima.
// returns the ssh output, if in layer, and an error if any
func ShowSSH(profileID string, layer bool, format string) (string, bool, error) {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "show-ssh", "--format", format, profileID)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("error retrieving ssh config: %w", err)
	}

	var port int
	if layer {
		port, _ = UbuntuSSHPort(profileID)
	}

	out := buf.String()
	switch format {
	case "config":
		out = replaceSSHConfig(out, profileID, port)
	case "cmd", "args":
		out = replaceSSHCmd(out, profileID, port)
	default:
		return "", false, fmt.Errorf("unsupported format '%v'", format)
	}

	return out, port > 0, nil
}

func replaceSSHCmd(cmd string, name string, port int) string {
	var out []string

	for _, s := range strings.Fields(cmd) {
		if strings.HasPrefix(s, "ControlPath=") {
			s = "ControlPath=" + strconv.Quote(filepath.Join(config.Dir(), "ssh.sock"))
		}
		if port > 0 && strings.HasPrefix(s, "Port=") {
			s = "Port=" + strconv.Itoa(port)
		}
		out = append(out, s)
	}

	if out[len(out)-1] == "lima-"+name {
		out[len(out)-1] = "127.0.0.1"
	}

	return strings.Join(out, " ")
}
func replaceSSHConfig(conf string, name string, port int) string {
	var out bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(conf))
	for scanner.Scan() {
		line := scanner.Text()

		switch {

		case strings.HasPrefix(strings.TrimSpace(line), "ControlPath "):
			pad := line[:strings.Index(line, "C")]
			line = pad + "ControlPath " + strconv.Quote(filepath.Join(config.Dir(), "ssh.sock"))

		case strings.HasPrefix(line, "Host "):
			line = "Host " + name

		case port > 0 && strings.HasPrefix(line, "Port "):
			pad := line[:strings.Index(line, "P")]
			line = pad + "Port " + strconv.Itoa(port)
		}

		_, _ = fmt.Fprintln(&out, line)
	}
	return out.String()
}

func toUserFriendlyName(name string) string {
	if name == "colima" {
		name = "default"
	}
	return strings.TrimPrefix(name, "colima-")
}

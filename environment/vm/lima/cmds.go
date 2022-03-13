package lima

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/cli"
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
	IPAddress string `json:"-"`
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
			i.IPAddress = getIPAddress(i.Name, "lima0")
		}

		// rename to local friendly names
		i.Name = toUserFriendlyName(i.Name)

		instances = append(instances, i)
	}

	return instances, nil
}

func getIPAddress(profile, interfaceName string) string {
	var buf bytes.Buffer
	// TODO: this should be cleaner
	cmd := cli.Command("limactl", "shell", profile, "sh", "-c",
		`ifconfig `+interfaceName+` | grep "inet addr:" | awk -F' ' '{print $2}' | awk -F':' '{print $2}'`)
	cmd.Stdout = &buf

	_ = cmd.Run()
	return strings.TrimSpace(buf.String())
}

// ShowSSH runs the show-ssh command in Lima.
func ShowSSH(name, format string) error {
	var buf bytes.Buffer
	cmd := cli.Command("limactl", "show-ssh", "--format", format, name)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error retrieving ssh config: %w", err)
	}

	// TODO: this is a lazy approach, edge cases may not be covered
	from := "lima-" + name
	to := name
	out := strings.ReplaceAll(buf.String(), from, to)

	fmt.Println(out)
	return nil
}

func toUserFriendlyName(name string) string {
	if name == "colima" {
		name = "default"
	}
	return strings.TrimPrefix(name, "colima-")
}

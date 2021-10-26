package docker

import (
	"encoding/json"
	"fmt"
	"github.com/abiosoft/colima/config"
	"path/filepath"
)

func (d dockerRuntime) setupInVM() error {
	// install in VM
	err := d.guest.Run("sudo", "apt-get", "-y", "install", "apt-transport-https", "ca-certificates", "curl", "gnupg", "lsb-release")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}
	err = d.guest.Run("bash", "-c", "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}
	err = d.guest.Run("bash", "-c", "echo \"deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}
	err = d.guest.Run("sudo", "apt-get", "update")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}
	err = d.guest.Run("sudo", "apt-get", "-y", "install", "docker-ce", "docker-ce-cli", "containerd.io")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}

	return d.createDaemonFile(daemonFile())
}

func (d dockerRuntime) fixUserPermission() error {
	user, err := d.guest.User()
	if err != nil {
		return fmt.Errorf("error retrieving user in the VM: %w", err)
	}
	if err := d.guest.Run("sudo", "usermod", "-aG", "docker", user); err != nil {
		return fmt.Errorf("error fixing user permission: %w", err)
	}
	return nil
}

var daemonJson struct {
	Features struct {
		BuildKit bool `json:"buildkit"`
	} `json:"features"`
	ExecOpts []string `json:"exec-opts"`
}

func init() {
	// enable buildkit by default.
	daemonJson.Features.BuildKit = true
	// k3s needs cgroupfs
	daemonJson.ExecOpts = append(daemonJson.ExecOpts, "native.cgroupdriver=cgroupfs")
}

func daemonFile() string {
	return filepath.Join(config.Dir(), "docker", "daemon.json")
}

func (d dockerRuntime) createDaemonFile(fileName string) error {
	// ensure directory
	if err := d.host.RunQuiet("mkdir", "-p", filepath.Dir(fileName)); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	b, err := json.MarshalIndent(daemonJson, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling deamon.json: %w", err)
	}
	return d.host.Write(fileName, string(b))
}

func (d dockerRuntime) setupDaemonFile() error {
	log := d.Logger()

	daemonFile := daemonFile()

	// check daemon.json or create default
	if _, err := d.host.Stat(daemonFile); err != nil {
		log.Warnln("daemon.json not found, falling back to default")
		if err := d.createDaemonFile(daemonFile); err != nil {
			return fmt.Errorf("error creating daemon.json: %w", err)
		}
	}

	daemonFileInVM := filepath.Join(config.CacheDir(), "daemon.json")

	// copy to vm, cache directory is shared by host and vm and guaranteed to be mounted.
	if err := d.host.RunQuiet("cp", daemonFile, daemonFileInVM); err != nil {
		return fmt.Errorf("error copying daemon.json to VM: %w", err)
	}

	// copy to location in VM
	if err := d.guest.RunQuiet("sudo", "mkdir", "-p", "/etc/docker"); err != nil {
		return fmt.Errorf("error setting up default config: %w", err)
	}

	if err := d.guest.RunQuiet("sudo", "cp", daemonFileInVM, "/etc/docker/daemon.json"); err != nil {
		return fmt.Errorf("error copying deamon.json: %w", err)
	}

	// config changed, restart is a must
	if d.Running() {
		return d.guest.RunQuiet("sudo", "service", "docker", "stop")
	}

	return nil
}

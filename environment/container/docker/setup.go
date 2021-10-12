package docker

import (
	"encoding/json"
	"fmt"
	"github.com/abiosoft/colima/config"
	"path/filepath"
)

func (d dockerRuntime) setupSocketSymlink() error {
	log := d.Logger()
	// remove existing socket (if any)
	log.Println("sudo password may be required to set up docker socket")
	err := d.host.RunInteractive("sudo", "rm", "-rf", socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	// create new symlink
	err = d.host.RunInteractive("sudo", "ln", "-s", socketSymlink(), socket)
	if err != nil {
		return fmt.Errorf("error setting up socket: %w", err)
	}
	return nil
}

func (d dockerRuntime) setupInVM() error {
	// install in VM
	err := d.guest.Run("sudo", "apt", "-y", "install", "docker.io")
	if err != nil {
		return fmt.Errorf("error installing in VM: %w", err)
	}

	return nil
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

func (d dockerRuntime) createDaemonFile(fileName string) error {
	b, err := json.MarshalIndent(daemonJson, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling deamon.json: %w", err)
	}
	return d.host.Write(fileName, string(b))
}

func (d dockerRuntime) setupDaemonFile() error {
	log := d.Logger()
	daemonFile := filepath.Join(config.Dir(), "docker", "daemon.json")

	// ensure config directory
	if err := d.host.RunQuiet("mkdir", "-p", filepath.Dir(daemonFile)); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

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

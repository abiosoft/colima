package podman

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
)

// setupInVM downloads the latest podman version in the VM via apt.
// since the main repo contains old versions of podman, we add the kubic project repo
func (p podmanRuntime) setupInVM() error {
	// TODO: Not working yet!
	// source /etc/os-release for version_id
	// cmd := "cat /etc/os-release | grep VERSION_ID"
	// versionID, err := p.guest.RunOutput("bash", "-c", cmd)
	// if err != nil {
	// 	return fmt.Errorf("Can't source /etc/release: %v", err)
	// }
	// versionIDSlice := strings.Split(versionID, "=")
	// err = os.Setenv(versionIDSlice[0], versionIDSlice[1])
	// if err != nil {
	// 	return fmt.Errorf("Can't set Environment variable %v: %v", versionIDSlice[0], err)
	// }

	// add kubic repo into sources.list.d
	cmd := "echo deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_21.04/ / | sudo tee /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list"
	err := p.guest.Run("bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("Can't add kubic repo to sources.list.d dir: %v", err)
	}

	// download kubic key and install in apt keyring
	cmd = "curl -L 'https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_21.04/Release.key' | sudo apt-key add -"
	err = p.guest.Run("bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("Can't install kubic apt key: %v", err)
	}
	// update and install podman
	err = p.guest.Run("sudo", "apt", "update")
	if err != nil {
		return fmt.Errorf("error updating apt in VM: %v", err)
	}

	// Interactive, because fuse.conf wants to be overwritten
	err = p.guest.RunInteractive("sudo", "apt", "-y", "install", "podman")
	if err != nil {
		return fmt.Errorf("error installing podman in VM: %v", err)
	}

	// fix service unit to run in endless loop
	// cmd = "sudo sed -i 's#system service#system service -t 0 unix:///home/hoehl.linux/podman.sock#g' /lib/systemd/system/podman.service"
	// err = p.guest.Run("bash", "-c", cmd)
	// if err != nil {
	// return fmt.Errorf("error changing system unit file of podman in VM: %v", err)
	// }
	// err = p.guest.Run("sudo", "systemctl", "daemon-reload")
	// if err != nil {
	// return fmt.Errorf("error reloading systemd daemon in VM: %w", err)
	// }
	return nil
}

func (p podmanRuntime) isInstalled() bool {
	err := p.guest.RunQuiet("command", "-v", "podman")
	return err == nil
}

// podman system connection add --identity ~/.ssh/dev_rsa testing ssh://root@server.fubar.com:2222
// createPodmanConnectionOnHost adds the remote connection to the host podman environment and sets it to default
func (p podmanRuntime) createPodmanConnectionOnHost(port int, vmName string) error {
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("Couldn't read current user: %v", err)
	}
	sshURI := fmt.Sprintf("ssh://%v@localhost:%v", user.Username, port)
	homeDir, _ := os.UserHomeDir()

	socketPath := fmt.Sprintf("/run/user/%v/podman/podman.sock", user.Uid)
	return p.host.Run("podman", "system", "connection", "add", "--socket-path", socketPath, "-d", vmName, "--identity", fmt.Sprintf("%v/.lima/_config/user", homeDir), sshURI)
}

type podmanConnections struct {
	Name     string
	Identity string
	URI      string
}

func (p podmanRuntime) checkIfPodmanRemoteConnectionIsValid(sshPort string) (bool, error) {
	connectionJSON, err := p.host.RunOutput("podman", "system", "connection", "list", "--format", "json")
	if err != nil {
		return false, fmt.Errorf("Can't get podman connections on host: %v", err)
	}
	var connections []podmanConnections
	err = json.Unmarshal([]byte(connectionJSON), &connections)
	if err != nil {
		return false, fmt.Errorf("Can't unmarshal podman connections json: %v", err)
	}
	re := regexp.MustCompile(`colima.*`)

	for _, connection := range connections {
		if re.MatchString(connection.Name) {
			//ssh://foo@bar:SSHPORT
			return strings.Split(connection.URI, ":")[2] == sshPort, nil
		}
	}
	return false, nil
}

func (p podmanRuntime) checkIfPodmanSocketIsRunning() (bool, error) {
	cmd := "ps -elf | grep 'podman system socket' | wc -l"
	output, err := p.guest.RunOutput("bash", "-c", cmd)
	if err != nil {
		return false, fmt.Errorf("Can't check if podman Socket is running in VM: %v", err)
	}
	wordCount, err := strconv.Atoi(output)
	// bash command itself and grep command should be excluded
	return wordCount != 2, nil
}

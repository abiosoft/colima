package podman

import (
	"encoding/json"
	"fmt"
	"os/user"
	"regexp"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/config"
)

// podman system connection add --identity ~/.ssh/dev_rsa testing ssh://root@server.fubar.com:2222
// setupConnection adds the remote connection to the host podman environment and sets it to default
func (p podmanRuntime) setupConnection() error {
	user, err := user.Current()
	if err != nil {
		return err
	}
	port, err := p.getSSHPortFromLimactl()
	if err != nil {
		return err
	}

	sshURI := fmt.Sprintf("ssh://%v@localhost:%v", user.Username, port)

	return p.host.Run(
		"podman",
		"system",
		"connection",
		"add",
		"--default",
		"--socket-path",
		"/run/podman/podman.sock",
		config.Profile().ID,
		"--identity",
		fmt.Sprintf("%v/.lima/_config/user", user.HomeDir),
		sshURI,
	)
}
func (p podmanRuntime) checkIfPodmanRemoteConnectionIsValid(sshPort int, vmName string) (bool, error) {
	connectionJSON, err := p.host.RunOutput("podman", "system", "connection", "list", "--format", "json")
	if err != nil {
		return false, fmt.Errorf("Can't get podman connections on host: %v", err)
	}
	var connections []podmanConnections
	err = json.Unmarshal([]byte(connectionJSON), &connections)
	if err != nil {
		return false, fmt.Errorf("Can't unmarshal podman connections json: %v", err)
	}
	re := regexp.MustCompile(fmt.Sprintf(`%v.*`, vmName))
	for _, connection := range connections {
		if re.MatchString(connection.Name) {
			//ssh://foo@bar:SSHPORT
			return strings.Split(connection.URI, ":")[2] == strconv.Itoa(sshPort), nil
		}
	}
	return false, nil
}

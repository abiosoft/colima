package limautil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
)

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
	profileID = config.ProfileFromName(profileID).ID

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

const sshConfigFile = "ssh.config"

// sshConfig is the ssh configuration file for a Colima profile.
type sshConfig string

// Contents returns the content of the SSH config file.
func (s sshConfig) Contents() (string, error) {
	profile := config.ProfileFromName(string(s))
	b, err := os.ReadFile(s.File())
	if err != nil {
		return "", fmt.Errorf("error retrieving Lima SSH config file for profile '%s': %w", strings.TrimPrefix(profile.DisplayName, "lima"), err)
	}
	return string(b), nil
}

// File returns the path to the SSH config file.
func (s sshConfig) File() string {
	profile := config.ProfileFromName(string(s))
	return filepath.Join(profile.LimaInstanceDir(), sshConfigFile)
}

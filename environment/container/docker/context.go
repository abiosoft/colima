package docker

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"path/filepath"
)

// HostSocketFile returns the path to the docker socket on host.
func HostSocketFile() string { return filepath.Join(config.Dir(), "docker.sock") }

func (d dockerRuntime) isContextCreated() bool {
	command := fmt.Sprintf(`docker context ls -q | grep "^%s$"`, config.Profile().ID)
	return d.host.RunQuiet("sh", "-c", command) == nil
}

func (d dockerRuntime) setupContext() error {
	if d.isContextCreated() {
		return nil
	}

	profile := config.Profile().ID

	return d.host.Run("docker", "context", "create", profile,
		"--description", profile,
		"--docker", "host=unix://"+HostSocketFile(),
	)
}

func (d dockerRuntime) useContext() error {
	return d.host.Run("docker", "context", "use", config.Profile().ID)
}

func (d dockerRuntime) teardownContext() error {
	if !d.isContextCreated() {
		return nil
	}

	return d.host.Run("docker", "context", "rm", "--force", config.Profile().ID)
}

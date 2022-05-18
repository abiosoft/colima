package docker

import (
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

// HostSocketFile returns the path to the docker socket on host.
func HostSocketFile() string { return filepath.Join(config.Dir(), "docker.sock") }
func LegacyDefaultHostSocketFile() string {
	return filepath.Join(filepath.Dir(config.Dir()), "docker.sock")
}

func (d dockerRuntime) isContextCreated() bool {
	command := fmt.Sprintf(`docker context ls -q | grep "^%s$"`, config.CurrentProfile().ID)
	return d.host.RunQuiet("sh", "-c", command) == nil
}

func (d dockerRuntime) setupContext() error {
	if d.isContextCreated() {
		return nil
	}

	profile := config.CurrentProfile()

	return d.host.Run("docker", "context", "create", profile.ID,
		"--description", profile.DisplayName,
		"--docker", "host=unix://"+HostSocketFile(),
	)
}

func (d dockerRuntime) useContext() error {
	return d.host.Run("docker", "context", "use", config.CurrentProfile().ID)
}

func (d dockerRuntime) teardownContext() error {
	if !d.isContextCreated() {
		return nil
	}

	return d.host.Run("docker", "context", "rm", "--force", config.CurrentProfile().ID)
}

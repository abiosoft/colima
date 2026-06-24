package docker

import (
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

var configDir = func() string { return config.CurrentProfile().ConfigDir() }

// HostSocketFile returns the path to the docker socket on host.
func HostSocketFile() string { return filepath.Join(configDir(), "docker.sock") }
func LegacyDefaultHostSocketFile() string {
	return filepath.Join(filepath.Dir(configDir()), "docker.sock")
}

func (d dockerRuntime) contextCreated() bool {
	return d.host.RunQuiet("docker", "context", "inspect", config.CurrentProfile().ID) == nil
}

func (d dockerRuntime) setupContext() error {
	if d.contextCreated() {
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
	if !d.contextCreated() {
		return nil
	}

	return d.host.Run("docker", "context", "rm", "--force", config.CurrentProfile().ID)
}

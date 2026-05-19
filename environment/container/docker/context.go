package docker

import (
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
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

	// In native mode, use the system Docker socket directly
	socketPath := HostSocketFile()
	if conf, err := configmanager.LoadInstance(); err == nil && conf.VMType == "native" {
		socketPath = "/var/run/docker.sock"
	}

	return d.host.Run("docker", "context", "create", profile.ID,
		"--description", profile.DisplayName,
		"--docker", "host=unix://"+socketPath,
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

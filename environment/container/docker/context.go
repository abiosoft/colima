package docker

import (
	"fmt"
	"github.com/abiosoft/colima/config"
)

func (d dockerRuntime) isContextCreated() bool {
	command := fmt.Sprintf(`docker context ls -q | grep "^%s$"`, config.Profile())
	return d.host.RunQuiet("sh", "-c", command) == nil
}

func (d dockerRuntime) setupContext() error {
	if d.isContextCreated() {
		return nil
	}

	profile := config.Profile().Name

	return d.host.Run("docker", "context", "create", profile,
		"--description", profile,
		"--docker", "host=unix://"+socketSymlink(),
	)
}

func (d dockerRuntime) useContext() error {
	return d.host.Run("docker", "context", "use", config.Profile().Name)
}

func (d dockerRuntime) teardownContext() error {
	if !d.isContextCreated() {
		return nil
	}

	return d.host.Run("docker", "context", "rm", "--force", config.Profile().Name)
}

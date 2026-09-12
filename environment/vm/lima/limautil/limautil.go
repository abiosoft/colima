package limautil

import (
	"os/exec"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util/osutil"
)

// EnvLimaHome is the environment variable for the Lima directory.
const EnvLimaHome = "LIMA_HOME"

// EnvLimaDrivers is the environment variable for the path to external Lima drivers.
const EnvLimaDrivers = "LIMA_DRIVERS_PATH"

// EnvLimaWorkDir is the Lima guest working-directory override.
// When set on the host to a path that does not exist inside the VM, `lima`
// fails with `cd: ... No such file or directory` (see #1048).
const EnvLimaWorkDir = "LIMA_WORKDIR"

// LimactlCommand is the limactl command.
const LimactlCommand = "limactl"

// Limactl prepares a limactl command.
func Limactl(args ...string) *exec.Cmd {
	cmd := cli.Command(LimactlCommand, args...)
	// Drop host LIMA_WORKDIR so lima/limactl do not cd into a host-only path.
	cmd.Env = append(cmd.Env, osutil.EnvironWithout(EnvLimaWorkDir)...)
	cmd.Env = append(cmd.Env, EnvLimaHome+"="+config.LimaDir())
	return cmd
}

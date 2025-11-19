package limautil

import (
	"os"
	"os/exec"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
)

// EnvLimaHome is the environment variable for the Lima directory.
const EnvLimaHome = "LIMA_HOME"

// EnvLimaDrivers is the environment variable for the path to external Lima drivers.
const EnvLimaDrivers = "LIMA_DRIVERS_PATH"

// LimactlCommand is the limactl command.
const LimactlCommand = "limactl"

// Limactl prepares a limactl command.
func Limactl(args ...string) *exec.Cmd {
	cmd := cli.Command(LimactlCommand, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, EnvLimaHome+"="+config.LimaDir())
	return cmd
}

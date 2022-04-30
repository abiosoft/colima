package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	_ "github.com/abiosoft/colima/cmd"        // for other commands
	_ "github.com/abiosoft/colima/cmd/daemon" // for vmnet daemon
	_ "github.com/abiosoft/colima/embedded"   // for embedded assets
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/gvproxy"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
)

func main() {
	_, cmd := filepath.Split(os.Args[0])
	switch cmd {
	case "qemu-system-x86_64", "qemu-system-aarch64":
		qemuWrapper(cmd)
	default:
		root.Execute()
	}
}
func qemuWrapper(qemu string) {
	// remove colima wrapper from path
	binDir := filepath.Join(config.WrapperDir(), "bin")
	_ = os.Setenv("PATH", util.RemoveFromPath(os.Getenv("PATH"), binDir))

	info := gvproxy.Info()

	// check if qemu is meant to run by lima
	// decided by -pidfile flag
	qemuRunning := false
	for i := 0; i < len(os.Args)-1; i++ {
		if os.Args[i] == "-pidfile" {
			qemuRunning = true
			break
		}
	}

	args := os.Args[1:] // forward all args
	var fd *os.File

	gvproxyEnabled, _ := strconv.ParseBool(os.Getenv(gvproxy.SubProcessEnvVar))

	if qemuRunning && gvproxyEnabled {
		conn, err := net.Dial("unix", info.Socket.File())
		if err != nil {
			logrus.Fatal(err)
		}
		fd, err = conn.(*net.UnixConn).File()
		if err != nil {
			logrus.Fatal(err)
		}

		args = append(args,
			"-netdev", "socket,id=vlan,fd=3",
			"-device", "virtio-net-pci,netdev=vlan,mac="+info.MacAddress,
		)
	}

	cmd := exec.Command(qemu, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if fd != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, fd)
	}

	_ = cmd.Run()

}

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

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process/gvproxy"
	"github.com/sirupsen/logrus"
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
	if profile := os.Getenv(config.SubprocessProfileEnvVar); profile != "" {
		config.SetProfile(profile)
	}

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

	err := cmd.Run()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
		}
		os.Exit(1)
	}

}

package main

import (
	"fmt"
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
	"github.com/abiosoft/colima/daemon/process/vmnet"
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

	gvproxyInfo := gvproxy.Info()
	vmnetInfo := vmnet.Info()

	// check if qemu is meant to run by lima
	// decided by -pidfile flag
	qemuRunning := false
	for _, arg := range os.Args {
		if arg == "-pidfile" {
			qemuRunning = true
			break
		}
	}

	args := os.Args[1:] // forward all args
	var extraFiles []*os.File

	gvproxyEnabled, _ := strconv.ParseBool(os.Getenv(gvproxy.SubProcessEnvVar))
	vmnetEnabled, _ := strconv.ParseBool(os.Getenv(vmnet.SubProcessEnvVar))

	if qemuRunning && gvproxyEnabled {
		// vmnet should come first as it would be added by Lima and would have the fd 3

		// vmnet
		if vmnetEnabled {
			fd := os.NewFile(3, vmnetInfo.Socket.File())
			extraFiles = append(extraFiles, fd)
		}

		// gvproxy
		{
			conn, err := net.Dial("unix", gvproxyInfo.Socket.File())
			if err != nil {
				logrus.Fatal(fmt.Errorf("error connecting to gvproxy socket: %w", err))
			}
			fd, err := conn.(*net.UnixConn).File()
			if err != nil {
				logrus.Fatal(fmt.Errorf("error retrieving fd for gvproxy socket: %w", err))
			}
			extraFiles = append(extraFiles, fd)
		}

		// gvproxy fd
		fd := strconv.Itoa(2 + len(extraFiles))
		args = append(args,
			"-netdev", "socket,id=vlan,fd="+fd,
			"-device", "virtio-net-pci,netdev=vlan,mac="+gvproxyInfo.MacAddress,
		)
	}

	cmd := exec.Command(qemu, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if len(extraFiles) > 0 {
		cmd.ExtraFiles = append(cmd.ExtraFiles, extraFiles...)
	}

	err := cmd.Run()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
		}
		os.Exit(1)
	}

}

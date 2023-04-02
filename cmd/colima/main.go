package main

import (
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

	gvproxyEnabled, _ := strconv.ParseBool(os.Getenv(gvproxy.SubProcessEnvVar))

	if qemuRunning && gvproxyEnabled {
		args = append(args,
			"-netdev", "stream,id=vlan,addr.type=unix,addr.path="+gvproxyInfo.Socket.File(),
			"-device", "virtio-net-pci,netdev=vlan,mac="+gvproxyInfo.MacAddress,
		)
	}

	cmd := exec.Command(qemu, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.ExitCode())
		}
		os.Exit(1)
	}

}

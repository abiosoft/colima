package docker

import (
	_ "embed"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
	"os"
	"path/filepath"
)

const (
	socket = "/var/run/docker.sock"
)

//go:embed socket.sh
var socketForwardingScript string

func socketForwardingScriptFile() string {
	return filepath.Join(config.Dir(), "socket.sh")
}

func socketSymlink() string {
	return filepath.Join(config.Dir(), "docker.sock")
}

func createSocketForwardingScript(vmUser string, sshPort int) error {
	scriptFile := socketForwardingScriptFile()
	// do nothing if previously created
	if stat, err := os.Stat(scriptFile); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("forwarding script: directory not expected at '%s'", scriptFile)
		}
	}

	// write socket script to file
	var values = struct {
		SocketFile string
		SSHPort    int
		VMUser     string
	}{SocketFile: socketSymlink(), SSHPort: sshPort, VMUser: vmUser}

	err := util.WriteTemplate(socketForwardingScript, scriptFile, values)
	if err != nil {
		return fmt.Errorf("error writing socket forwarding script: %w", err)
	}

	// make executable
	if err := os.Chmod(scriptFile, 0755); err != nil {
		return err
	}

	return nil
}

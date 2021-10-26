package docker

import (
	_ "embed"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
	"os"
	"path/filepath"
)

//go:embed socket.sh
var socketForwardingScript string

func socketForwardingScriptFile() string {
	return filepath.Join(config.Dir(), "socket.sh")
}

func socketSymlink() string {
	return filepath.Join(config.Dir(), "docker.sock")
}

func CreateSocketForwardingScript(vmUser string, sshPort int, remoteSocketPath string, socketFile string) error {
	scriptFile := socketForwardingScriptFile()
	// do nothing if previously created
	if stat, err := os.Stat(scriptFile); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("forwarding script: directory not expected at '%s'", scriptFile)
		}
	}

	// write socket script to file
	var values = struct {
		SocketFile   string
		SSHPort      int
		VMUser       string
		RemoteSocket string
	}{SocketFile: socketFile, SSHPort: sshPort, VMUser: vmUser, RemoteSocket: remoteSocketPath}

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

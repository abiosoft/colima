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

//go:embed socket.plist
var socketLaunchdScript string

func socketForwardingScriptFile() string {
	return filepath.Join(config.Dir(), "socket.sh")
}

func createSocketForwardingScript() error {
	scriptFile := socketForwardingScriptFile()
	// do nothing if previously created
	if stat, err := os.Stat(scriptFile); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("forwarding script: directory not expected at '%s'", scriptFile)
		}
		return nil
	}

	// write socket script to file
	var values = struct {
		SocketFile string
		SSHPort    int
	}{SocketFile: dockerSocketSymlink(), SSHPort: config.SSHPort()}

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

func launchdDir() string {
	home := util.HomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func launchdPackage() string {
	return "com.abiosoft." + config.AppName()
}

func launchdFile() string {
	return filepath.Join(launchdDir(), launchdPackage())
}

func createLaunchdScript() error {
	if err := os.MkdirAll(launchdDir(), 0755); err != nil {
		return fmt.Errorf("error creating launchd directory: %w", err)
	}
	packageName := launchdPackage()
	launchdScriptFile := filepath.Join(launchdDir(), packageName)

	if stat, err := os.Stat(launchdScriptFile); err != nil {
		if stat.IsDir() {
			return fmt.Errorf("launchd file: directory not expected at '%s'", launchdScriptFile)
		}
		return nil
	}

	var values = struct {
		Package    string
		SocketFile string
	}{Package: packageName, SocketFile: socketForwardingScriptFile()}

	if err := util.WriteTemplate(socketLaunchdScript, launchdScriptFile, values); err != nil {
		return fmt.Errorf("error writing launchd file: %w", err)
	}

	return nil
}

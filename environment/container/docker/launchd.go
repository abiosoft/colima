package docker

import (
	_ "embed"
	"fmt"
	"github.com/abiosoft/colima/util"
	"os"
	"path/filepath"
)

//go:embed socket.plist
var launchdScript string

type launchAgent string

func (l launchAgent) String() string { return string(l) }

func (l launchAgent) File() string {
	return filepath.Join(l.Dir(), string(l)+".plist")
}

// Dir returns the user launchd directory.
func (l launchAgent) Dir() string {
	home := util.HomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func createLaunchdScript(launchd launchAgent) error {
	if err := os.MkdirAll(launchd.Dir(), 0755); err != nil {
		return fmt.Errorf("error creating launchd directory: %w", err)
	}
	if stat, err := os.Stat(launchd.File()); err == nil {
		if stat.IsDir() {
			return fmt.Errorf("launchd file: directory not expected at '%s'", launchd.File())
		}
		return nil
	}

	var values = struct {
		Package    string
		SocketFile string
	}{Package: launchd.String(), SocketFile: socketForwardingScriptFile()}

	if err := util.WriteTemplate(launchdScript, launchd.File(), values); err != nil {
		return fmt.Errorf("error writing launchd file: %w", err)
	}

	return nil
}

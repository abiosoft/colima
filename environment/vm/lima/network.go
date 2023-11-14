package lima

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
)

func (l *limaVM) writeNetworkFile() error {
	networkFile := limautil.NetworkFile()
	embeddedFile, err := embedded.Read("network/networks.yaml")
	if err != nil {
		return fmt.Errorf("error reading embedded network config file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(networkFile), 0755); err != nil {
		return fmt.Errorf("error creating Lima config directory: %w", err)
	}
	if err := os.WriteFile(networkFile, embeddedFile, 0755); err != nil {
		return fmt.Errorf("error writing Lima network config file: %w", err)
	}
	return nil
}

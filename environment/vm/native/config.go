package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	log "github.com/sirupsen/logrus"
)

const configFileName = "native.json"

// configFilePath returns the path to the native KV config store.
// Lima stores this inside the VM at /etc/colima/colima.json.
// Native mode stores it in the profile directory on the host.
func (n nativeVM) configFilePath() string {
	return filepath.Join(config.CurrentProfile().ConfigDir(), configFileName)
}

func (n nativeVM) loadConfig() map[string]string {
	obj := map[string]string{}
	b, err := os.ReadFile(n.configFilePath())
	if err != nil {
		log.Tracef("error reading native config file: %v", err)
		return obj
	}

	_ = json.Unmarshal(b, &obj)
	return obj
}

// Get retrieves a configuration value.
func (n nativeVM) Get(key string) string {
	if val, ok := n.loadConfig()[key]; ok {
		return val
	}
	return ""
}

// Set stores a configuration value.
func (n nativeVM) Set(key, value string) error {
	obj := n.loadConfig()
	obj[key] = value

	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshalling settings to json: %w", err)
	}

	dir := filepath.Dir(n.configFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	if err := os.WriteFile(n.configFilePath(), b, 0644); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}

package apple

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
)

// stateFile returns the path to the Apple Container state file on host.
// Unlike Lima, Apple Container has no guest filesystem to store config,
// so state is persisted in the profile's config directory on the host.
func stateFile() string {
	return filepath.Join(config.CurrentProfile().ConfigDir(), "apple.json")
}

func (a appleVM) getConf() map[string]string {
	obj := map[string]string{}
	b, err := os.ReadFile(stateFile())
	if err != nil {
		return obj
	}

	_ = json.Unmarshal(b, &obj)

	return obj
}

// Get retrieves a configuration value.
func (a appleVM) Get(key string) string {
	if val, ok := a.getConf()[key]; ok {
		return val
	}
	return ""
}

// Set sets a configuration value.
func (a appleVM) Set(key, value string) error {
	obj := a.getConf()
	obj[key] = value

	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshalling settings to json: %w", err)
	}

	dir := filepath.Dir(stateFile())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	if err := os.WriteFile(stateFile(), b, 0o644); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}
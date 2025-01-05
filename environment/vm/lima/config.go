package lima

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

const configFile = "/etc/colima/colima.json"

func (l limaVM) getConf() map[string]string {
	log := l.Logger(context.Background())

	obj := map[string]string{}
	b, err := l.Read(configFile)
	if err != nil {
		log.Trace(fmt.Errorf("error reading config file: %w", err))

		return obj
	}

	// we do not care if it fails
	_ = json.Unmarshal([]byte(b), &obj)

	return obj
}
func (l limaVM) Get(key string) string {
	if val, ok := l.getConf()[key]; ok {
		return val
	}

	return ""
}

func (l limaVM) Set(key, value string) error {
	obj := l.getConf()
	obj[key] = value

	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshalling settings to json: %w", err)
	}

	if err := l.Run("sudo", "mkdir", "-p", filepath.Dir(configFile)); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	if err := l.Write(configFile, b); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}

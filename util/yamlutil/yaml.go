package yamlutil

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// WriteYAML encodes struct to file as YAML.
func WriteYAML(value interface{}, file string) error {
	b, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("error encoding YAML: %w", err)
	}

	return os.WriteFile(file, b, 0644)
}

package util

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"text/template"
)

// WriteTemplate writes template with body to file after applying values.
func WriteTemplate(body string, file string, values interface{}) error {
	b, err := ParseTemplate(body, values)
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}

// ParseTemplate parses template with body and values and returns the resulting bytes.
func ParseTemplate(body string, values interface{}) ([]byte, error) {
	t, err := template.New("").Parse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}

	var b bytes.Buffer
	if err := t.Execute(&b, values); err != nil {
		return nil, fmt.Errorf("error executing template: %w", err)
	}

	return b.Bytes(), err
}

// WriteYAML encodes struct to file as YAML.
func WriteYAML(value interface{}, file string) error {
	b, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("error encoding YAML: %w", err)
	}

	return os.WriteFile(file, b, 0644)
}

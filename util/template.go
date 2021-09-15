package util

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// WriteTemplate writes template with body to file after applying values.
func WriteTemplate(body string, file string, values interface{}) error {
	t, err := template.New("").Parse(body)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	var b bytes.Buffer
	if err := t.Execute(&b, values); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	return os.WriteFile(file, b.Bytes(), 0644)
}

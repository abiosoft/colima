package embedded

import (
	"embed"
)

//go:embed network
var FS embed.FS

func read(file string) ([]byte, error) { return FS.ReadFile(file) }

// Read reads the content of file
func Read(file string) ([]byte, error) { return read(file) }

// ReadString reads the content of file as string
func ReadString(file string) (string, error) {
	b, err := read(file)
	return string(b), err
}

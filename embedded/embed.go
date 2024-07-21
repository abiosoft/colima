package embedded

import (
	"embed"
)

//go:embed network k3s defaults images
var fs embed.FS

// FS returns the underlying embed.FS
func FS() embed.FS { return fs }

func read(file string) ([]byte, error) { return fs.ReadFile(file) }

// Read reads the content of file
func Read(file string) ([]byte, error) { return read(file) }

// ReadString reads the content of file as string
func ReadString(file string) (string, error) {
	b, err := read(file)
	return string(b), err
}

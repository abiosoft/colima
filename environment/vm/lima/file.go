package lima

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/colima/environment"
)

func (l limaVM) Read(fileName string) (string, error) {
	s, err := l.RunOutput("sudo", "cat", fileName)
	if err != nil {
		return "", fmt.Errorf("cannot read file '%s': %w", fileName, err)
	}
	return s, err
}

func (l *limaVM) Write(fileName string, body []byte) error {
	var stdin = bytes.NewReader(body)
	dir := filepath.Dir(fileName)
	if err := l.RunQuiet("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error creating directory '%s': %w", dir, err)
	}
	return l.RunWith(stdin, nil, "sudo", "sh", "-c", "cat > "+fileName)
}

func (l *limaVM) Stat(fileName string) (os.FileInfo, error) {
	return newFileInfo(l, fileName)
}

var _ os.FileInfo = (*fileInfo)(nil)

type fileInfo struct {
	isDir   bool
	modTime time.Time
	mode    fs.FileMode
	name    string
	size    int64
}

func newFileInfo(guest environment.GuestActions, filename string) (fileInfo, error) {
	// "%s,%a,%Y,%F" -> size, permission, modified time, type
	stat, err := guest.RunOutput("sudo", "stat", "-c", "%s,%a,%Y,%F", filename)
	if err != nil {
		return fileInfo{}, statError(filename, err)
	}
	return parseFileInfo(filename, stat)
}

func parseFileInfo(filename, stat string) (fileInfo, error) {
	stats := strings.Split(strings.TrimSpace(stat), ",")
	if len(stats) != 4 {
		return fileInfo{}, fmt.Errorf("cannot parse stat output for file %q: expected four fields", filename)
	}
	size, err := strconv.ParseInt(stats[0], 10, 64)
	if err != nil {
		return fileInfo{}, fmt.Errorf("cannot parse size for file %q: %w", filename, err)
	}
	// GNU stat's %a is an octal permission string, such as "644".
	mode, err := strconv.ParseUint(stats[1], 8, 32)
	if err != nil {
		return fileInfo{}, fmt.Errorf("cannot parse permissions for file %q: %w", filename, err)
	}
	unix, err := strconv.ParseInt(stats[2], 10, 64)
	if err != nil {
		return fileInfo{}, fmt.Errorf("cannot parse modification time for file %q: %w", filename, err)
	}
	return fileInfo{
		name:    filepath.Base(filename),
		size:    size,
		mode:    fs.FileMode(mode),
		modTime: time.Unix(unix, 0),
		isDir:   stats[3] == "directory",
	}, nil
}

func statError(filename string, err error) error {
	return fmt.Errorf("cannot stat file '%s': %w", filename, err)
}

// IsDir implements fs.FileInfo
func (f fileInfo) IsDir() bool { return f.isDir }

// ModTime implements fs.FileInfo
func (f fileInfo) ModTime() time.Time { return f.modTime }

// Mode implements fs.FileInfo
func (f fileInfo) Mode() fs.FileMode { return f.mode }

// Name implements fs.FileInfo
func (f fileInfo) Name() string { return f.name }

// Size implements fs.FileInfo
func (f fileInfo) Size() int64 { return f.size }

// Sys implements fs.FileInfo
func (fileInfo) Sys() any { return nil }

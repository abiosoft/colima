package apple

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Read reads a file from the container.
func (a appleVM) Read(fileName string) (string, error) {
	s, err := a.RunOutput("cat", fileName)
	if err != nil {
		return "", fmt.Errorf("cannot read file '%s': %w", fileName, err)
	}
	return s, err
}

// Write writes a file to the container.
func (a *appleVM) Write(fileName string, body []byte) error {
	var stdin = bytes.NewReader(body)
	dir := filepath.Dir(fileName)
	if err := a.RunQuiet("mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error creating directory '%s': %w", dir, err)
	}
	return a.RunWith(stdin, nil, "sh", "-c", "cat > "+fileName)
}

// Stat returns file information for a file in the container.
func (a *appleVM) Stat(fileName string) (os.FileInfo, error) {
	return newFileInfo(a, fileName)
}

var _ os.FileInfo = (*fileInfo)(nil)

type fileInfo struct {
	isDir   bool
	modTime time.Time
	mode    fs.FileMode
	name    string
	size    int64
}

func newFileInfo(a *appleVM, filename string) (fileInfo, error) {
	info := fileInfo{}
	// "%s,%a,%Y,%F" -> size, permission, modified time, type
	stat, err := a.RunOutput("stat", "-c", "%s,%a,%Y,%F", filename)
	if err != nil {
		return info, statError(filename, err)
	}
	stats := strings.Split(stat, ",")
	if len(stats) < 4 {
		return info, statError(filename, err)
	}
	info.name = filename
	info.size, _ = strconv.ParseInt(stats[0], 10, 64)
	info.mode = func() fs.FileMode {
		mode, _ := strconv.ParseUint(stats[1], 10, 32)
		return fs.FileMode(mode)
	}()
	info.modTime = func() time.Time {
		unix, _ := strconv.ParseInt(stats[2], 10, 64)
		return time.Unix(unix, 0)
	}()
	info.isDir = stats[3] == "directory"

	return info, nil
}

func statError(filename string, err error) error {
	return fmt.Errorf("cannot stat file '%s': %w", filename, err)
}

// IsDir implements fs.FileInfo.
func (f fileInfo) IsDir() bool { return f.isDir }

// ModTime implements fs.FileInfo.
func (f fileInfo) ModTime() time.Time { return f.modTime }

// Mode implements fs.FileInfo.
func (f fileInfo) Mode() fs.FileMode { return f.mode }

// Name implements fs.FileInfo.
func (f fileInfo) Name() string { return f.name }

// Size implements fs.FileInfo.
func (f fileInfo) Size() int64 { return f.size }

// Sys implements fs.FileInfo.
func (fileInfo) Sys() any { return nil }

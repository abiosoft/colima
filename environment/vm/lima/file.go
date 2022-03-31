package lima

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
)

func (l limaVM) Read(fileName string) (string, error) {
	s, err := l.RunOutput("sudo", "sh", "-c", "cat", fileName)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %s", fileName)
	}
	return s, err
}

func (l *limaVM) Write(fileName, body string) error {
	// utlilise host cache path
	tmpName := filepath.Join(config.CacheDir(), util.SHA256Hash(fileName))

	if err := l.host.Write(tmpName, body); err != nil {
		return fmt.Errorf("cannot write file to vm: %w", err)
	}

	// transfer to desired location in the vm
	if err := l.RunQuiet("sudo", "cp", tmpName, fileName); err != nil {
		return fmt.Errorf("error file %s in vm: %w", fileName, err)
	}
	_ = l.host.RunQuiet("rm", "-f", tmpName)

	return nil
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
	statErr := fmt.Errorf("cannot stat file: %s", filename)
	info := fileInfo{}
	// "%s,%a,%Y,%F" -> size, permission, modified time, type
	stat, err := guest.RunOutput("sudo", "stat", "-c", "%s,%a,%Y,%F", filename)
	if err != nil {
		return info, statErr
	}
	stats := strings.Split(stat, ",")
	if len(stats) < 4 {
		return info, statErr
	}
	info.name = filename
	info.size, _ = strconv.ParseInt(stats[0], 10, 64)
	info.mode = func() fs.FileMode {
		mode, _ := strconv.Atoi(stats[1])
		return fs.FileMode(mode)
	}()
	info.modTime = func() time.Time {
		unix, _ := strconv.ParseInt(stats[2], 10, 64)
		return time.Unix(unix, 0)
	}()
	info.isDir = stats[3] == "directory"

	return info, nil
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

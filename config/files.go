package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/sirupsen/logrus"
)

// requiredDir is a directory that must exist on the filesystem
type requiredDir struct {
	once sync.Once

	// dir is a func to enable deferring the value of the directory
	// until execution time.
	// if dir() returns an error, a fatal error is triggered.
	dir func() (string, error)

	computedDir *string
}

// Dir returns the directory path.
// It ensures the directory is created on the filesystem by calling
// `mkdir` prior to returning the directory path.
func (r *requiredDir) Dir() string {
	if r.computedDir != nil {
		return *r.computedDir
	}

	dir, err := r.dir()
	if err != nil {
		logrus.Fatal(fmt.Errorf("cannot fetch required directory: %w", err))
	}

	r.once.Do(func() {
		if err := fsutil.MkdirAll(dir, 0755); err != nil {
			logrus.Fatal(fmt.Errorf("cannot make required directory: %w", err))
		}
	})

	r.computedDir = &dir
	return dir
}

var (
	configBaseDir = requiredDir{
		dir: func() (string, error) {
			dir := os.Getenv("COLIMA_HOME")
			if _, err := os.Stat(dir); err == nil {
				return dir, nil
			}

			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(dir, ".colima")
			_, err = os.Stat(dir)
			// TODO: remove macOS when QEMU_SYSTEM_ARCH is handled properly upstream.
			if err == nil || util.MacOS() {
				return dir, nil
			}
			// else
			dir = os.Getenv("XDG_CONFIG_HOME")
			if dir != "" {
				return filepath.Join(dir, "colima"), nil
			}
			// else
			dir, err = os.UserConfigDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "colima"), nil
		},
	}

	cacheDir = requiredDir{
		dir: func() (string, error) {
			dir := os.Getenv("XDG_CACHE_HOME")
			if dir != "" {
				return filepath.Join(dir, "colima"), nil
			}
			// else
			dir, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "colima"), nil
		},
	}

	templatesDir = requiredDir{
		dir: func() (string, error) {
			dir, err := configBaseDir.dir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "_templates"), nil
		},
	}

	limaDir = requiredDir{
		dir: func() (string, error) {
			// if LIMA_HOME env var is set, obey it.
			if dir := os.Getenv("LIMA_HOME"); dir != "" {
				return dir, nil
			}

			dir, err := configBaseDir.dir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "_lima"), nil
		},
	}
)

// CacheDir returns the cache directory.
func CacheDir() string { return cacheDir.Dir() }

// TemplatesDir returns the templates' directory.
func TemplatesDir() string { return templatesDir.Dir() }

// LimaDir returns Lima directory.
func LimaDir() string { return limaDir.Dir() }

const configFileName = "colima.yaml"

// SSHConfigFile returns the path to generated ssh config.
func SSHConfigFile() string { return filepath.Join(configBaseDir.Dir(), "ssh_config") }

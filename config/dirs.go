package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/osutil"
	"github.com/abiosoft/colima/util/shautil"
	"github.com/sirupsen/logrus"
)

// requiredDir is a directory that must exist on the filesystem
type requiredDir struct {
	once sync.Once
	// dir is a func to enable deferring the value of the directory
	// until execution time.
	// if dir() returns an error, a fatal error is triggered.
	dir func() (string, error)
}

// Dir returns the directory path.
// It ensures the directory is created on the filesystem by calling
// `mkdir` prior to returning the directory path.
func (r *requiredDir) Dir() string {
	dir, err := r.dir()
	if err != nil {
		logrus.Fatal(fmt.Errorf("cannot fetch required directory: %w", err))
	}

	r.once.Do(func() {
		if err := fsutil.MkdirAll(dir, 0755); err != nil {
			logrus.Fatal(fmt.Errorf("cannot make required directory: %w", err))
		}
	})

	return dir
}

var (
	configBaseDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(dir, ".colima")
			_, err = os.Stat(dir)
			if err == nil {
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

	configDir = requiredDir{
		dir: func() (string, error) {
			dir, err := configBaseDir.dir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, profile.ShortName), nil
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

	wrapperDir = requiredDir{
		dir: func() (string, error) {
			dir, err := configBaseDir.dir()
			if err != nil {
				return "", err
			}
			// generate unique directory for the current binary
			uniqueDir := shautil.SHA1(osutil.Executable())
			return filepath.Join(dir, "_wrapper", uniqueDir.String()), nil
		},
	}
)

// Dir returns the configuration directory.
func Dir() string { return configDir.Dir() }

// File returns the path to the config file.
func File() string { return configFile() }

// CacheDir returns the cache directory.
func CacheDir() string { return cacheDir.Dir() }

// TemplatesDir returns the templates' directory.
func TemplatesDir() string { return templatesDir.Dir() }

// WrapperDir returns the qemu wrapper directory.
func WrapperDir() string { return wrapperDir.Dir() }

const ConfigFileName = "colima.yaml"

func configFile() string { return filepath.Join(configDir.Dir(), ConfigFileName) }

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/abiosoft/colima/util"
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			logrus.Fatal(fmt.Errorf("cannot make required directory: %w", err))
		}
	})

	return dir
}

var (
	configDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, ".colima", profile.ShortName), nil
		},
	}

	cacheDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, "colima"), nil
		},
	}

	templatesDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(dir, ".colima", "_templates"), nil
		},
	}

	wrapperDir = requiredDir{
		dir: func() (string, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			// generate unique directory for the current binary
			uniqueDir := util.SHA1(util.Executable())
			return filepath.Join(dir, ".colima", "_wrapper", uniqueDir.String()), nil
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

const configFileName = "colima.yaml"

func configFile() string { return filepath.Join(configDir.Dir(), configFileName) }

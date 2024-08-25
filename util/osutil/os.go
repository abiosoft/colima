package osutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// EnvVar is environment variable
type EnvVar string

// Exists checks if the environment variable has been set.
func (e EnvVar) Exists() bool {
	_, ok := os.LookupEnv(string(e))
	return ok
}

// Bool returns the environment variable value as boolean.
func (e EnvVar) Bool() bool {
	ok, _ := strconv.ParseBool(e.Val())
	return ok
}

// Bool returns the environment variable value.
func (e EnvVar) Val() string {
	return os.Getenv(string(e))
}

const EnvColimaBinary = "COLIMA_BINARY"

// Executable returns the path name for the executable that started
// the current process.
func Executable() string {
	e, err := func(s string) (string, error) {
		// prioritize env var in case this is a nested process
		if e := os.Getenv(EnvColimaBinary); e != "" {
			return e, nil
		}

		if filepath.IsAbs(s) {
			return s, nil
		}

		e, err := exec.LookPath(s)
		if err != nil {
			return "", fmt.Errorf("error looking up '%s' in PATH: %w", s, err)
		}

		abs, err := filepath.Abs(e)
		if err != nil {
			return "", fmt.Errorf("error computing absolute path of '%s': %w", e, err)
		}

		return abs, nil
	}(os.Args[0])

	if err != nil {
		// this should never happen, thereby it is safe to do
		logrus.Traceln(fmt.Errorf("cannot detect current running executable: %w", err))
		logrus.Traceln("falling back to first CLI argument")
		return os.Args[0]
	}

	return e
}

// Socket is a unix socket
type Socket string

// Unix returns the unix address for the socket.
func (s Socket) Unix() string { return "unix://" + s.File() }

// File returns the file path for the socket.
func (s Socket) File() string { return strings.TrimPrefix(string(s), "unix://") }

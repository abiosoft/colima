package osutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

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
		logrus.Warnln(fmt.Errorf("cannot detect current running executable: %w", err))
		logrus.Warnln("falling back to first CLI argument")
		return os.Args[0]
	}

	return e
}

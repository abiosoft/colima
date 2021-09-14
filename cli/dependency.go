package cli

import (
	"fmt"
	"strings"
)

func IsInstalled(programs []string) error {
	var missing []string
	check := func(p string) error {
		return Run("command", "-v", p)
	}
	for _, p := range programs {
		if check(p) != nil {
			missing = append(missing, p)
		}
	}
	return fmt.Errorf("%s not found, run 'brew install %s' to install", strings.Join(missing, ", "), strings.Join(missing, " "))
}

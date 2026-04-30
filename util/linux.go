package util

import "runtime"

// Linux returns if the current OS is Linux.
func Linux() bool { return runtime.GOOS == "linux" }

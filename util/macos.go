package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
)

// MacOS returns if the current OS is macOS.
func MacOS() bool {
	return runtime.GOOS == "darwin"
}

// MacOS13OrNewer returns if the current OS is macOS 13 or newer.
func MacOS13OrNewerOnM1() bool {
	return runtime.GOARCH == "arm64" && MacOS13OrNewer()
}

// MacOS13OrNewer returns if the current OS is macOS 13 or newer.
func MacOS13OrNewer() bool {
	if !MacOS() {
		return false
	}
	ver, err := macOSProductVersion()
	if err != nil {
		logrus.Warnln(fmt.Errorf("error retrieving macOS version: %w", err))
		return false
	}

	cver, err := semver.NewVersion("13.0.0")
	if err != nil {
		logrus.Warnln(fmt.Errorf("error parsing version: %w", err))
		return false
	}

	return cver.Compare(*ver) <= 0
}

// M3 returns if the current device is an Apple M3 device.
func M3() bool {
	var resp struct {
		SPHardwareDataType []struct {
			ChipType string `json:"chip_type"`
		} `json:"SPHardwareDataType"`
	}

	var buf bytes.Buffer
	cmd := cli.Command("system_profiler", "-json", "SPHardwareDataType")
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		logrus.Trace(fmt.Errorf("error retriving chip version: %w", err))
		return false
	}

	if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
		logrus.Trace(fmt.Errorf("error decoding system_profiler response: %w", err))
		return false
	}

	if len(resp.SPHardwareDataType) == 0 {
		return false
	}

	chipType := strings.ToUpper(resp.SPHardwareDataType[0].ChipType)
	return strings.Contains(chipType, "M3")
}

// RosettaRunning checks if Rosetta process is running.
func RosettaRunning() bool {
	if !MacOS() {
		return false
	}
	cmd := cli.Command("pgrep", "oahd")
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
}

// macOSProductVersion returns the host's macOS version.
func macOSProductVersion() (*semver.Version, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	// output is like "12.3.1\n"
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %v: %w", cmd.Args, err)
	}
	verTrimmed := strings.TrimSpace(string(b))
	// macOS 12.4 returns just "12.4\n"
	for strings.Count(verTrimmed, ".") < 2 {
		verTrimmed += ".0"
	}
	verSem, err := semver.NewVersion(verTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse macOS version %q: %w", verTrimmed, err)
	}
	return verSem, nil
}

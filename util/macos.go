package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
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
func MacOS13OrNewerOnArm() bool {
	return runtime.GOARCH == "arm64" && MacOS13OrNewer()
}

// MacOS13OrNewer returns if the current OS is macOS 13 or newer.
func MacOS13OrNewer() bool { return minMacOSVersion("13.0.0") }

// MacOS15OrNewer returns if the current OS is macOS 15 or newer.
func MacOS15OrNewer() bool { return minMacOSVersion("15.0.0") }

// MacOSNestedVirtualizationSupported returns if the current device supports nested virtualization.
func MacOSNestedVirtualizationSupported() bool {
	return IsMxOrNewer(3) && MacOS15OrNewer()
}

func minMacOSVersion(version string) bool {
	if !MacOS() {
		return false
	}
	ver, err := macOSProductVersion()
	if err != nil {
		logrus.Warnln(fmt.Errorf("error retrieving macOS version: %w", err))
		return false
	}

	cver, err := semver.NewVersion(version)
	if err != nil {
		logrus.Warnln(fmt.Errorf("error parsing version: %w", err))
		return false
	}

	return cver.Compare(*ver) <= 0
}

// IsMxOrNewer returns true if the machine is Apple Silicon M{n} where n >= min
// e.g. IsMxOrNewer(3) returns true for M3, M4, M5, ...
func IsMxOrNewer(min int) bool {
	chip, err := chipDetector.GetChipType()
	if err != nil {
		logrus.Trace(fmt.Errorf("error getting chip type: %w", err))
		return false
	}
	n, ok := parseMNumber(chip)
	if !ok {
		return false
	}
	return n >= min
}

// chipTypeDetector fetches the chip type string from the host.
type chipTypeDetector interface {
	GetChipType() (string, error)
}

// systemProfilerChipDetector is the production implementation that calls
// `system_profiler -json SPHardwareDataType`.
type systemProfilerChipDetector struct{}

func (d systemProfilerChipDetector) GetChipType() (string, error) {
	if !MacOS() {
		return "", fmt.Errorf("not macOS")
	}
	var resp struct {
		SPHardwareDataType []struct {
			ChipType string `json:"chip_type"`
		} `json:"SPHardwareDataType"`
	}

	var buf bytes.Buffer
	cmd := cli.Command("system_profiler", "-json", "SPHardwareDataType")
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error retrieving chip version: %w", err)
	}

	if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
		return "", fmt.Errorf("error decoding system_profiler response: %w", err)
	}

	if len(resp.SPHardwareDataType) == 0 {
		return "", fmt.Errorf("no SPHardwareDataType in response")
	}

	return resp.SPHardwareDataType[0].ChipType, nil
}

// chipDetector is the instance used by IsMx/IsMxOrNewer. Tests can replace
// this with a fake implementation.
var chipDetector chipTypeDetector = systemProfilerChipDetector{}

var mRe = regexp.MustCompile(`\bM(\d+)\b`)

func parseMNumber(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	matches := mRe.FindStringSubmatch(s)
	if len(matches) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return n, true
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

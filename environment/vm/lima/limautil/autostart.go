package limautil

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
)

// AutostartCondition is when an instance should be started automatically.
type AutostartCondition string

const (
	// AutostartLogin starts the instance when the user logs in.
	AutostartLogin AutostartCondition = "login"
	// AutostartBoot starts the instance at system boot, before any user logs in.
	// macOS only, and requires privileges to write to /Library/LaunchDaemons.
	AutostartBoot AutostartCondition = "boot"
)

// minAutostartVersion is the minimum Lima version usable for autostart.
//
// A Colima instance lives under ~/.colima/_lima rather than the default ~/.lima, so the
// generated launchd/systemd unit has to carry LIMA_HOME. That was added in Lima v2.3.0
// (lima-vm/lima#5489); before it, the unit resolved the default directory at boot and
// never found the instance.
var minAutostartVersion = *semver.New("2.3.0")

// parseVersion extracts the version from the output of `limactl --version`,
// which is of the form "limactl version 2.3.0".
func parseVersion(output string) (*semver.Version, error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, fmt.Errorf("unexpected output from `%s --version`: %q", LimactlCommand, output)
	}
	return semver.NewVersion(strings.TrimPrefix(fields[len(fields)-1], "v"))
}

// Version returns the version of the limactl binary.
func Version() (*semver.Version, error) {
	var buf bytes.Buffer
	cmd := Limactl("--version")
	cmd.Stdout = &buf
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error retrieving lima version: %w", err)
	}
	return parseVersion(buf.String())
}

// checkAutostartSupported reports whether the installed Lima can autostart a Colima
// instance. A development build is rejected, as its version sorts below the release.
func checkAutostartSupported() error {
	version, err := Version()
	if err != nil {
		return err
	}
	if version.LessThan(minAutostartVersion) {
		return fmt.Errorf("autostart requires Lima v%s or newer, found v%s", minAutostartVersion, version)
	}
	return nil
}

// warnColimaProvision warns when the instance has provision scripts that Colima runs
// itself. The generated unit invokes `limactl start`, so only the scripts Colima passes
// through to Lima run on an automatic start.
func warnColimaProvision() {
	conf, err := configmanager.LoadInstance()
	if err != nil {
		return // not fatal, the instance may not have been started yet
	}
	for _, p := range conf.Provision {
		if p.IsColimaMode() {
			logrus.Warnf("provision scripts with mode %q or %q do not run on an automatic start, "+
				"as the instance is started by Lima rather than by Colima",
				config.ProvisionModeAfterBoot, config.ProvisionModeReady)
			return
		}
	}
}

// EnableAutostart registers the instance of the current profile to start
// automatically, on the given condition.
func EnableAutostart(condition AutostartCondition) error {
	if err := checkAutostartSupported(); err != nil {
		return err
	}
	warnColimaProvision()
	cmd := Limactl("autostart", "enable", config.CurrentProfile().ID, "--condition", string(condition))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error enabling autostart: %w", err)
	}
	return nil
}

// DisableAutostart unregisters the instance of the current profile from
// automatic startup.
func DisableAutostart() error {
	cmd := Limactl("autostart", "disable", config.CurrentProfile().ID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error disabling autostart: %w", err)
	}
	return nil
}

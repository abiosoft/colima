package systemctl

import "github.com/abiosoft/colima/environment"

// Runner is the subset of environment.GuestActions that Systemctl requires.
// Using a narrow interface makes Systemctl easier to test and more loosely coupled.
type Runner interface {
	Run(args ...string) error
	RunQuiet(args ...string) error
}

// compile-time check: environment.GuestActions satisfies runner.
var _ Runner = (environment.GuestActions)(nil)

// Systemctl provides a typed wrapper for running systemctl commands in the guest VM.
type Systemctl struct {
	runner Runner
}

// New creates a new Systemctl instance backed by the given guest.
func New(guest Runner) Systemctl {
	return Systemctl{runner: guest}
}

// Start starts a systemd service.
func (s Systemctl) Start(service string) error {
	return s.runner.Run("sudo", "systemctl", "start", service)
}

// Restart restarts a systemd service.
func (s Systemctl) Restart(service string) error {
	return s.runner.Run("sudo", "systemctl", "restart", service)
}

// Stop stops a systemd service. If force is true, it is killed immediately without graceful shutdown.
func (s Systemctl) Stop(service string, force bool) error {
	verb := "stop"
	if force {
		verb = "kill"
	}
	return s.runner.Run("sudo", "systemctl", verb, service)
}

// Active returns whether a systemd service is currently active.
func (s Systemctl) Active(service string) bool {
	return s.runner.RunQuiet("systemctl", "is-active", service) == nil
}

// DaemonReload reloads the systemd manager configuration.
func (s Systemctl) DaemonReload() error {
	return s.runner.Run("sudo", "systemctl", "daemon-reload")
}

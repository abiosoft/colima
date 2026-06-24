package process

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/config"

	"github.com/abiosoft/colima/environment"
)

func CtxKeyDaemon() any { return struct{ key string }{key: "colima_daemon"} }

// Process is a background process managed by the daemon.
type Process interface {
	// Name for the background process
	Name() string
	// Start starts the background process.
	// The process is expected to terminate when ctx is done.
	Start(ctx context.Context) error
	// Alive checks if the process is the alive.
	Alive(ctx context.Context) error
	// Dependencies are requirements for start to succeed.
	// root should be true if root access is required for
	// installing any of the dependencies.
	Dependencies() (deps []Dependency, root bool)
}

// Dir is the directory for daemon files.
func Dir() string { return filepath.Join(config.CurrentProfile().ConfigDir(), "daemon") }

// Dependency is a requirement to be fulfilled before a process can be started.
type Dependency interface {
	Installed() bool
	Install(environment.HostActions) error
}

// Dependencies returns the dependencies for the processes.
// root returns if root access is required
func Dependencies(processes ...Process) (deps Dependency, root bool) {
	// check rootful for user info message
	rootful := false
	for _, p := range processes {
		deps, root := p.Dependencies()
		for _, dep := range deps {
			if !dep.Installed() && root {
				rootful = true
				break
			}
		}
	}

	return processDeps(processes), rootful
}

type processDeps []Process

func (p processDeps) Installed() bool {
	for _, process := range p {
		deps, _ := process.Dependencies()
		for _, d := range deps {
			if !d.Installed() {
				return false
			}
		}
	}

	return true
}

func (p processDeps) Install(host environment.HostActions) error {
	for _, process := range p {
		deps, _ := process.Dependencies()
		for _, d := range deps {
			if !d.Installed() {
				if err := d.Install(host); err != nil {
					return fmt.Errorf("error occurred installing dependencies for '%s': %w", process.Name(), err)
				}
			}
		}
	}

	return nil
}

package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
)

// Name is container runtime name.
const Name = "apple"

var _ environment.Container = (*appleRuntime)(nil)

func init() {
	environment.RegisterContainer(Name, newRuntime, false)
}

type appleRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain
}

// newRuntime creates a new Apple Container runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &appleRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

func (a appleRuntime) Name() string {
	return Name
}

func (a appleRuntime) Provision(ctx context.Context) error {
	chain := a.Init(ctx)
	log := a.Logger(ctx)

	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	// Check if Apple Container is supported on this system
	chain.Add(func() error {
		if !util.MacOS15OrNewer() {
			return fmt.Errorf("Apple Container requires macOS 15 or newer")
		}
		return nil
	})

	// Install containerization framework if needed
	chain.Add(func() error {
		// Check if containerization framework is available
		if err := a.guest.RunQuiet("which", "containerization"); err != nil {
			log.Warnln("containerization framework not found, attempting to install...")
			// Note: In a real implementation, you would install the containerization framework
			// This is a placeholder for the actual installation logic
			return fmt.Errorf("containerization framework installation not implemented yet")
		}
		return nil
	})

	// Configure Apple Container settings
	chain.Add(func() error {
		// Create necessary directories and configuration files
		if err := a.guest.RunQuiet("sudo", "mkdir", "-p", "/etc/containerization"); err != nil {
			log.Warnln(err)
		}
		
		// Apply Apple Container configuration if provided
		if conf.Apple != nil {
			if err := a.applyConfiguration(conf.Apple); err != nil {
				log.Warnln(fmt.Errorf("error applying Apple Container configuration: %w", err))
			}
		}
		
		return nil
	})

	// Set up context for Apple Container
	chain.Add(a.setupContext)
	if conf.AutoActivate() {
		chain.Add(a.useContext)
	}

	return chain.Exec()
}

func (a appleRuntime) Start(ctx context.Context) error {
	chain := a.Init(ctx)

	// Start the Apple Container service
	chain.Retry("", time.Second, 30, func(int) error {
		return a.guest.RunQuiet("sudo", "launchctl", "load", "-w", "/System/Library/LaunchDaemons/com.apple.containerization.plist")
	})

	// Wait for the service to be ready
	chain.Retry("", time.Second, 60, func(int) error {
		return a.guest.RunQuiet("containerization", "info")
	})

	// Ensure containerization is accessible
	chain.Add(func() error {
		if err := a.guest.RunQuiet("containerization", "info"); err == nil {
			return nil
		}
		ctx := context.WithValue(ctx, cli.CtxKeyQuiet, true)
		return a.guest.Restart(ctx)
	})

	return chain.Exec()
}

func (a appleRuntime) Running(ctx context.Context) bool {
	return a.guest.RunQuiet("containerization", "info") == nil
}

func (a appleRuntime) Stop(ctx context.Context) error {
	chain := a.Init(ctx)

	chain.Add(func() error {
		if !a.Running(ctx) {
			return nil
		}
		return a.guest.Run("sudo", "launchctl", "unload", "/System/Library/LaunchDaemons/com.apple.containerization.plist")
	})

	// Clear Apple Container context settings
	chain.Add(a.teardownContext)

	return chain.Exec()
}

func (a appleRuntime) Teardown(ctx context.Context) error {
	chain := a.Init(ctx)

	// Clear Apple Container context settings
	chain.Add(a.teardownContext)

	return chain.Exec()
}

func (a appleRuntime) Dependencies() []string {
	// Apple Container is built into macOS 15+, so no external dependencies
	return []string{}
}

func (a appleRuntime) Version(ctx context.Context) string {
	version, _ := a.host.RunOutput("containerization", "--version")
	return version
}

func (a *appleRuntime) Update(ctx context.Context) (bool, error) {
	// Apple Container is updated through macOS system updates
	// Return false to indicate no update was performed
	return false, nil
}

// setupContext sets up the Apple Container context
func (a *appleRuntime) setupContext() error {
	// Create context configuration for Apple Container
	// This would typically involve setting up the context file
	// that points to the Apple Container instance
	profile := config.CurrentProfile()
	
	// Create the context directory
	contextDir := filepath.Join(util.HomeDir(), ".colima", "contexts", profile.ID)
	if err := a.host.RunQuiet("mkdir", "-p", contextDir); err != nil {
		return fmt.Errorf("error creating context directory: %w", err)
	}
	
	// Create context configuration file
	contextConfig := fmt.Sprintf(`{
		"name": "%s",
		"type": "apple",
		"endpoint": "unix:///var/run/containerization.sock",
		"host": "localhost",
		"port": 22
	}`, profile.ID)
	
	contextFile := filepath.Join(contextDir, "config.json")
	if err := a.host.Write(contextFile, []byte(contextConfig)); err != nil {
		return fmt.Errorf("error writing context config: %w", err)
	}
	
	return nil
}

// useContext activates the Apple Container context
func (a *appleRuntime) useContext() error {
	// Set the current context to use Apple Container
	profile := config.CurrentProfile()
	
	// Create a symlink to the current context
	currentContext := filepath.Join(util.HomeDir(), ".colima", "contexts", "current")
	contextDir := filepath.Join(util.HomeDir(), ".colima", "contexts", profile.ID)
	
	// Remove existing symlink if it exists
	a.host.RunQuiet("rm", "-f", currentContext)
	
	// Create new symlink
	if err := a.host.Run("ln", "-s", contextDir, currentContext); err != nil {
		return fmt.Errorf("error creating context symlink: %w", err)
	}
	
	return nil
}

// applyConfiguration applies Apple Container configuration
func (a *appleRuntime) applyConfiguration(config map[string]any) error {
	// Convert configuration to JSON and write to the guest
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling configuration: %w", err)
	}
	
	// Write configuration to the guest
	if err := a.guest.Write("/etc/containerization/config.json", configJSON); err != nil {
		return fmt.Errorf("error writing configuration: %w", err)
	}
	
	// Reload configuration if containerization is running
	if a.Running(context.Background()) {
		if err := a.guest.RunQuiet("sudo", "launchctl", "reload", "/System/Library/LaunchDaemons/com.apple.containerization.plist"); err != nil {
			return fmt.Errorf("error reloading configuration: %w", err)
		}
	}
	
	return nil
}

// teardownContext removes the Apple Container context
func (a *appleRuntime) teardownContext() error {
	// Clean up the Apple Container context configuration
	profile := config.CurrentProfile()
	
	// Remove context directory
	contextDir := filepath.Join(util.HomeDir(), ".colima", "contexts", profile.ID)
	a.host.RunQuiet("rm", "-rf", contextDir)
	
	// Remove current context symlink if it points to this profile
	currentContext := filepath.Join(util.HomeDir(), ".colima", "contexts", "current")
	if linkTarget, err := a.host.RunOutput("readlink", currentContext); err == nil {
		if linkTarget == contextDir {
			a.host.RunQuiet("rm", "-f", currentContext)
		}
	}
	
	return nil
} 
package colima

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/abiosoft/colima/environment/vm/lima"
	"log"
	"path/filepath"
)

type App interface {
	Start(config.Config) error
	Stop() error
	Delete() error
	SSH(...string) error
	Status() error
	Version() error
	Runtime() (string, error)
}

var _ App = (*colimaApp)(nil)

// New creates a new app.
func New() (App, error) {
	guest := lima.New(host.New())
	if err := host.IsInstalled(guest); err != nil {
		return nil, fmt.Errorf("dependency check failed for VM: %w", err)
	}

	return &colimaApp{
		guest: guest,
	}, nil
}

type colimaApp struct {
	guest vm.VM
}

const runtimeConfigFile = "/etc/colima/runtime"

func (c colimaApp) Start(conf config.Config) error {
	log.Println("starting", config.AppName())

	var containers []container.Container
	{
		env, err := c.containerEnvironment(conf.Runtime)
		if err != nil {
			return err
		}
		containers = append(containers, env)
		// TODO add kubernetes
	}

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(conf); err != nil {
		return fmt.Errorf("error starting vm: %w", err)
	}

	// provision container runtime
	for _, cont := range containers {
		if err := cont.Provision(); err != nil {
			return fmt.Errorf("error provisioning %s: %w", cont.Name(), err)
		}
	}

	// start container runtimes
	for _, cont := range containers {
		if err := cont.Start(); err != nil {
			return fmt.Errorf("error starting %s: %w", cont.Name(), err)
		}
	}

	// persist runtime for future reference.
	if err := c.setRuntime(conf.Runtime); err != nil {
		return fmt.Errorf("error setting current runtime: %w", err)
	}

	log.Println("done")
	return nil
}

func (c colimaApp) Stop() error {
	log.Println("stopping", config.AppName())

	// the order for stop is:
	//   container stop -> vm stop

	// stop container runtimes
	if c.guest.Running() {
		containers, err := c.currentContainerEnvironments()
		if err != nil {
			log.Println(err)
		}
		for _, cont := range containers {
			if err := cont.Stop(); err != nil {
				// failure to stop a container runtime is not fatal
				// it is only meant for graceful shutdown.
				// the VM will shut down anyways.
				log.Println(fmt.Errorf("error stopping %s: %w", cont.Name(), err))
			}
		}
	}

	// stop vm
	// no need to check running status, it may be in a state that requires stopping.
	if err := c.guest.Stop(); err != nil {
		return fmt.Errorf("error stopping vm: %w", err)
	}

	log.Println("done")
	return nil
}

func (c colimaApp) Delete() error {
	y := cli.Prompt("are you sure you want to delete Colima and all settings")
	if !y {
		return nil
	}

	log.Println("deleting", config.AppName())

	// the order for teardown is:
	//   container teardown -> vm teardown

	// vm teardown would've sufficed but container provision
	// may have created configurations on the host.
	// it is essential to teardown containers as well.

	// teardown container runtimes
	if c.guest.Running() {
		containers, err := c.currentContainerEnvironments()
		if err != nil {
			log.Println(err)
		}
		for _, cont := range containers {
			if err := cont.Teardown(); err != nil {
				// failure here is not fatal
				log.Println(fmt.Errorf("error during teardown of %s: %w", cont.Name(), err))
			}
		}
	}

	// teardown vm
	if err := c.guest.Teardown(); err != nil {
		return fmt.Errorf("error during teardown of vm: %w", err)
	}

	// delete configs
	if err := config.Teardown(); err != nil {
		return fmt.Errorf("error deleting configs: %w", err)
	}

	log.Println("done")
	return nil
}

func (c colimaApp) SSH(args ...string) error {
	if !c.guest.Running() {
		return fmt.Errorf("%s not running", config.AppName())
	}

	return c.guest.RunInteractive(args...)
}

func (c colimaApp) Status() error {
	if !c.guest.Running() {
		return fmt.Errorf("%s is not running", config.AppName())
	}

	currentRuntime, err := c.currentRuntime()
	if err != nil {
		return err
	}

	fmt.Println(config.AppName(), "is running")
	fmt.Println("runtime:", currentRuntime)

	return nil
}

func (c colimaApp) Version() error {
	name := config.AppName()
	version := config.AppVersion()
	fmt.Println(name, "version", version)

	if c.guest.Running() {
		containerRuntimes, err := c.currentContainerEnvironments()
		if err != nil {
			return err
		}
		for _, cont := range containerRuntimes {
			fmt.Println()
			fmt.Println("runtime:", cont.Name())
			fmt.Println(cont.Version())
		}
	}

	return nil
}

func (c colimaApp) currentRuntime() (string, error) {
	if !c.guest.Running() {
		return "", fmt.Errorf("%s is not running", config.AppName())
	}

	r, err := c.guest.RunOutput("cat", runtimeConfigFile)
	if err != nil {
		return "", fmt.Errorf("error retrieving current runtime: %w", err)
	}

	return r, nil
}

func (c colimaApp) setRuntime(runtimeName string) error {
	if err := c.guest.Run("sudo", "mkdir", "-p", filepath.Dir(runtimeConfigFile)); err != nil {
		return fmt.Errorf("error saving runtime settings: %w", err)
	}
	if err := c.guest.Run("sudo", "sh", "-c", fmt.Sprintf(`echo "%s" > %s`, runtimeName, runtimeConfigFile)); err != nil {
		return fmt.Errorf("error saving runtime settings: %w", err)
	}

	return nil
}

func (c colimaApp) currentContainerEnvironments() ([]container.Container, error) {
	var containers []container.Container

	runtime, err := c.currentRuntime()
	if err != nil {
		return nil, err
	}

	env, err := c.containerEnvironment(runtime)
	if err != nil {
		return nil, err
	}

	containers = append(containers, env)

	// detect and add kubernetes

	return containers, nil
}

func (c colimaApp) containerEnvironment(runtime string) (container.Container, error) {
	env, err := container.New(runtime, c.guest.Host(), c.guest)
	if err != nil {
		return nil, fmt.Errorf("error initiating container runtime: %w", err)
	}
	if err := host.IsInstalled(env); err != nil {
		return nil, fmt.Errorf("dependency check failed for %s: %w", runtime, err)
	}

	return env, nil
}

func (c colimaApp) Runtime() (string, error) {
	return c.currentRuntime()
}
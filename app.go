package colima

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/runtime/container"
	"github.com/abiosoft/colima/runtime/container/docker"
	"github.com/abiosoft/colima/runtime/host"
	"github.com/abiosoft/colima/runtime/vm"
	"github.com/abiosoft/colima/runtime/vm/lima"
	"log"
)

type App interface {
	Start() error
	Stop() error
	Delete() error
	SSH(...string) error
}

var _ App = (*colimaApp)(nil)

// New creates a new app.
func New(c config.Config) (App, error) {
	guest := lima.New(host.New(), c)
	if err := host.IsInstalled(guest); err != nil {
		return nil, fmt.Errorf("dependency check failed for VM: %w", err)
	}

	dockerRuntime := docker.New(guest.Host(), guest)
	if err := host.IsInstalled(dockerRuntime); err != nil {
		return nil, fmt.Errorf("dependency check failed for docker: %w", err)
	}

	return &colimaApp{
		guest:      guest,
		containers: []container.Container{dockerRuntime},
	}, nil
}

type colimaApp struct {
	guest      vm.VM
	containers []container.Container
}

func (c colimaApp) Start() error {
	log.Println("starting", config.AppName())

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(); err != nil {
		return fmt.Errorf("error starting vm: %w", err)
	}

	// provision container runtimes
	for _, cont := range c.containers {
		if err := cont.Provision(); err != nil {
			return fmt.Errorf("error provisioning %s: %w", cont.Name(), err)
		}
	}

	// start container runtimes
	for _, cont := range c.containers {
		if err := cont.Start(); err != nil {
			return fmt.Errorf("error starting %s: %w", cont.Name(), err)
		}
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
		for _, cont := range c.containers {
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
	log.Println("deleting", config.AppName())

	// the order for teardown is:
	//   container teardown -> vm teardown

	// vm teardown would've sufficed but container provision
	// may have created configurations on the host.
	// it is essential to teardown containers as well.

	// teardown container runtimes
	if c.guest.Running() {
		for _, cont := range c.containers {
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

	log.Println("done")
	return nil
}

func (c colimaApp) SSH(args ...string) error {
	return c.guest.RunInteractive(args...)
}

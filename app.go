package colima

import (
	"fmt"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"log"
	"strconv"
)

type App interface {
	Active() bool
	Start(config.Config) error
	Stop() error
	Delete() error
	SSH(...string) error
	Status() error
	Version() error
	Runtime() (string, error)
	Kubernetes() (environment.Container, error)
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
	guest environment.VM
}

func (c colimaApp) Start(conf config.Config) error {
	log.Println("starting", config.Profile())

	var containers []environment.Container
	// runtime
	{
		env, err := c.containerEnvironment(conf.Runtime)
		if err != nil {
			return err
		}
		containers = append(containers, env)
	}
	// kubernetes
	if conf.Kubernetes.Enabled {
		env, err := c.containerEnvironment(kubernetes.Name)
		if err != nil {
			return err
		}
		containers = append(containers, env)
	}

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(conf); err != nil {
		return fmt.Errorf("error starting vm: %w", err)
	}

	// persist runtime for future reference.
	if err := c.setRuntime(conf.Runtime); err != nil {
		return fmt.Errorf("error setting current runtime: %w", err)
	}
	if err := c.setSSHPort(conf.VM.SSHPort); err != nil {
		return fmt.Errorf("error setting SSH port: %w", err)
	}
	// persist kubernetes version for future reference.
	if err := c.setKubernetesVersion(conf.Kubernetes.Version); err != nil {
		return fmt.Errorf("error setting kubernetes version: %w", err)
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

	log.Println("done")
	return nil
}

func (c colimaApp) Stop() error {
	log.Println("stopping", config.Profile())

	// the order for stop is:
	//   container stop -> vm stop

	// stop container runtimes
	if c.guest.Running() {
		containers, err := c.currentContainerEnvironments()
		if err != nil {
			log.Println(err)
		}

		// stop happens in reverse of start
		for i := len(containers) - 1; i >= 0; i-- {
			cont := containers[i]
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
	y := cli.Prompt("are you sure you want to delete " + config.Profile() + " and all settings")
	if !y {
		return nil
	}

	log.Println("deleting", config.Profile())

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
		return fmt.Errorf("%s not running", config.Profile())
	}

	return c.guest.RunInteractive(args...)
}

func (c colimaApp) Status() error {
	if !c.guest.Running() {
		return fmt.Errorf("%s is not running", config.Profile())
	}

	currentRuntime, err := c.currentRuntime()
	if err != nil {
		return err
	}

	fmt.Println(config.Profile(), "is running")
	fmt.Println("runtime:", currentRuntime)

	// kubernetes
	if k, err := c.Kubernetes(); err == nil && k.Version() != "" {
		fmt.Println("kubernetes: enabled")
	}

	return nil
}

func (c colimaApp) Version() error {
	name := config.Profile()
	version := config.AppVersion()
	fmt.Println(name, "version", version)

	if c.guest.Running() {
		containerRuntimes, err := c.currentContainerEnvironments()
		if err != nil {
			return err
		}

		var kube environment.Container
		for _, cont := range containerRuntimes {
			if cont.Name() == kubernetes.Name {
				kube = cont
				continue
			}
			fmt.Println()
			fmt.Println("runtime:", cont.Name())
			fmt.Println(cont.Version())
		}

		if kube != nil && kube.Version() != "" {
			fmt.Println()
			fmt.Println("kubernetes")
			fmt.Println(kube.Version())
		}
	}

	return nil
}

func (c colimaApp) currentRuntime() (string, error) {
	if !c.guest.Running() {
		return "", fmt.Errorf("%s is not running", config.Profile())
	}

	r := c.guest.Get(environment.ContainerRuntimeKey)
	if r == "" {
		return "", fmt.Errorf("error retrieving current runtime: empty value")
	}

	return r, nil
}

func (c colimaApp) setRuntime(runtime string) error {
	return c.guest.Set(environment.ContainerRuntimeKey, runtime)
}

func (c colimaApp) setKubernetesVersion(version string) error {
	return c.guest.Set(environment.KubernetesVersionKey, version)
}

func (c colimaApp) setSSHPort(sshPort int) error {
	return c.guest.Set(environment.SSHPortKey, strconv.Itoa(sshPort))
}

func (c colimaApp) currentContainerEnvironments() ([]environment.Container, error) {
	var containers []environment.Container

	// runtime
	{
		runtime, err := c.currentRuntime()
		if err != nil {
			return nil, err
		}
		env, err := c.containerEnvironment(runtime)
		if err != nil {
			return nil, err
		}
		containers = append(containers, env)
	}

	// detect and add kubernetes
	if k, err := c.containerEnvironment(kubernetes.Name); err == nil && k.Version() != "" {
		containers = append(containers, k)
	}

	return containers, nil
}

func (c colimaApp) containerEnvironment(runtime string) (environment.Container, error) {
	env, err := environment.NewContainer(runtime, c.guest.Host(), c.guest)
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

func (c colimaApp) Kubernetes() (environment.Container, error) {
	return c.containerEnvironment(kubernetes.Name)
}

func (c colimaApp) Active() bool {
	return c.guest.Running()
}

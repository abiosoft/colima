package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/environment/container/ubuntu"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	log "github.com/sirupsen/logrus"
)

type App interface {
	Active() bool
	Start(config.Config) error
	Stop(force bool) error
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
	ctx := context.WithValue(context.Background(), config.CtxKey(), conf)

	{
		runtime := conf.Runtime
		if conf.Kubernetes.Enabled {
			runtime += "+k3s"
		}
		log.Println("starting", config.Profile().DisplayName)
		log.Println("runtime:", runtime)
	}
	var containers []environment.Container
	// runtime
	{
		env, err := c.containerEnvironment(conf.Runtime)
		if err != nil {
			return err
		}
		containers = append(containers, env)
	}
	// kubernetes should come after required runtime
	if conf.Kubernetes.Enabled {
		env, err := c.containerEnvironment(kubernetes.Name)
		if err != nil {
			return err
		}
		containers = append(containers, env)
	}
	// ubuntu layer should come last
	if conf.Ubuntu {
		env, err := c.containerEnvironment(ubuntu.Name)
		if err != nil {
			return err
		}
		containers = append(containers, env)
	}

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(ctx, conf); err != nil {
		return fmt.Errorf("error starting vm: %w", err)
	}

	// provision and start container runtimes
	for _, cont := range containers {
		if err := cont.Provision(ctx); err != nil {
			return fmt.Errorf("error provisioning %s: %w", cont.Name(), err)
		}
		if err := cont.Start(ctx); err != nil {
			return fmt.Errorf("error starting %s: %w", cont.Name(), err)
		}
	}

	// persist the current runtime
	if err := c.setRuntime(conf.Runtime); err != nil {
		log.Error(fmt.Errorf("error persisting runtime settings: %w", err))
	}

	log.Println("done")
	return nil
}

func (c colimaApp) Stop(force bool) error {
	ctx := context.Background()
	log.Println("stopping", config.Profile().DisplayName)

	// the order for stop is:
	//   container stop -> vm stop

	// stop container runtimes if not a forceful shutdown
	if c.guest.Running() && !force {
		containers, err := c.currentContainerEnvironments()
		if err != nil {
			log.Warnln(fmt.Errorf("error retrieving runtimes: %w", err))
		}

		// stop happens in reverse of start
		for i := len(containers) - 1; i >= 0; i-- {
			cont := containers[i]
			if err := cont.Stop(ctx); err != nil {
				// failure to stop a container runtime is not fatal
				// it is only meant for graceful shutdown.
				// the VM will shut down anyways.
				log.Warnln(fmt.Errorf("error stopping %s: %w", cont.Name(), err))
			}
		}
	}

	// stop vm
	// no need to check running status, it may be in a state that requires stopping.
	if err := c.guest.Stop(ctx, force); err != nil {
		return fmt.Errorf("error stopping vm: %w", err)
	}

	log.Println("done")
	return nil
}

func (c colimaApp) Delete() error {
	ctx := context.Background()
	log.Println("deleting", config.Profile().DisplayName)

	// the order for teardown is:
	//   container teardown -> vm teardown

	// vm teardown would've sufficed but container provision
	// may have created configurations on the host.
	// it is thereby necessary to teardown containers as well.

	// teardown container runtimes
	if c.guest.Running() {
		containers, err := c.currentContainerEnvironments()
		if err != nil {
			log.Warnln(fmt.Errorf("error retrieving runtimes: %w", err))
		}
		for _, cont := range containers {
			if err := cont.Teardown(ctx); err != nil {
				// failure here is not fatal
				log.Warnln(fmt.Errorf("error during teardown of %s: %w", cont.Name(), err))
			}
		}
	}

	// teardown vm
	if err := c.guest.Teardown(ctx); err != nil {
		return fmt.Errorf("error during teardown of vm: %w", err)
	}

	// delete configs
	if err := configmanager.Teardown(); err != nil {
		return fmt.Errorf("error deleting configs: %w", err)
	}

	log.Println("done")
	return nil
}

func (c colimaApp) SSH(args ...string) error {
	if !c.guest.Running() {
		return fmt.Errorf("%s not running", config.Profile().DisplayName)
	}

	cmdArgs, inLayer, err := lima.ShowSSH(config.Profile().ID, "args")
	if err != nil {
		return fmt.Errorf("error getting ssh config: %w", err)
	}

	if !inLayer {
		return c.guest.RunInteractive(args...)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Debug(fmt.Errorf("cannot get working dir: %w", err))
	}

	if len(args) > 0 {
		args = append([]string{"-q", "-t", "127.0.0.1", "--"}, args...)
	} else if wd != "" {
		args = []string{"-q", "-t", "127.0.0.1", "--", "cd " + wd + " 2> /dev/null; bash --login"}
	}

	args = append(strings.Fields(cmdArgs), args...)
	return cli.CommandInteractive("ssh", args...).Run()

}

func (c colimaApp) Status() error {
	if !c.guest.Running() {
		return fmt.Errorf("%s is not running", config.Profile().DisplayName)
	}

	currentRuntime, err := c.currentRuntime()
	if err != nil {
		return err
	}

	log.Println(config.Profile().DisplayName, "is running")
	log.Println("arch:", c.guest.Arch())
	log.Println("runtime:", currentRuntime)
	if currentRuntime == docker.Name {
		log.Println("socket:", "unix://"+docker.HostSocketFile())
	}

	// kubernetes
	if k, err := c.Kubernetes(); err == nil && k.Running() {
		log.Println("kubernetes: enabled")
	}

	return nil
}

func (c colimaApp) Version() error {
	if !c.guest.Running() {
		return nil
	}

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
		fmt.Println("arch:", c.guest.Arch())
		fmt.Println(cont.Version())
	}

	if kube != nil && kube.Version() != "" {
		fmt.Println()
		fmt.Println(kubernetes.Name)
		fmt.Println(kube.Version())
	}

	return nil
}

func (c colimaApp) currentRuntime() (string, error) {
	if !c.guest.Running() {
		return "", fmt.Errorf("%s is not running", config.Profile().DisplayName)
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
	if k, err := c.containerEnvironment(kubernetes.Name); err == nil && k.Running() {
		containers = append(containers, k)
	}

	// detect and add ubuntu layer
	if u, err := c.containerEnvironment(ubuntu.Name); err == nil && u.Running() {
		containers = append(containers, u)
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

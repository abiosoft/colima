package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/docker/go-units"
	log "github.com/sirupsen/logrus"
)

type App interface {
	Active() bool
	Start(config.Config) error
	Stop(force bool) error
	Delete() error
	SSH(args ...string) error
	Status(extended bool) error
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
		log.Println("starting", config.CurrentProfile().DisplayName)
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

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(ctx, conf); err != nil {
		return fmt.Errorf("error starting vm: %w", err)
	}

	// provision and start container runtimes
	for _, cont := range containers {
		log := log.WithField("context", cont.Name())
		log.Println("provisioning ...")
		if err := cont.Provision(ctx); err != nil {
			return fmt.Errorf("error provisioning %s: %w", cont.Name(), err)
		}
		log.Println("starting ...")
		if err := cont.Start(ctx); err != nil {
			return fmt.Errorf("error starting %s: %w", cont.Name(), err)
		}
	}

	// persist the current runtime
	if err := c.setRuntime(conf.Runtime); err != nil {
		log.Error(fmt.Errorf("error persisting runtime settings: %w", err))
	}

	// persist the kubernetes config
	if err := c.setKubernetes(conf.Kubernetes); err != nil {
		log.Error(fmt.Errorf("error persisting kubernetes settings: %w", err))
	}

	log.Println("done")

	if err := generateSSHConfig(conf.SSHConfig); err != nil {
		log.Trace("error generating ssh_config: %w", err)
	}
	return nil
}

func (c colimaApp) Stop(force bool) error {
	ctx := context.Background()
	log.Println("stopping", config.CurrentProfile().DisplayName)

	// the order for stop is:
	//   container stop -> vm stop

	// stop container runtimes if not a forceful shutdown
	if c.guest.Running(ctx) && !force {
		containers, err := c.currentContainerEnvironments(ctx)
		if err != nil {
			log.Warnln(fmt.Errorf("error retrieving runtimes: %w", err))
		}

		// stop happens in reverse of start
		for i := len(containers) - 1; i >= 0; i-- {
			cont := containers[i]

			log := log.WithField("context", cont.Name())
			log.Println("stopping ...")

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

	if err := generateSSHConfig(false); err != nil {
		log.Trace("error generating ssh_config: %w", err)
	}
	return nil
}

func (c colimaApp) Delete() error {
	ctx := context.Background()
	log.Println("deleting", config.CurrentProfile().DisplayName)

	// the order for teardown is:
	//   container teardown -> vm teardown

	// vm teardown would've sufficed but container provision
	// may have created configurations on the host.
	// it is thereby necessary to teardown containers as well.

	// teardown container runtimes
	if c.guest.Running(ctx) {
		containers, err := c.currentContainerEnvironments(ctx)
		if err != nil {
			log.Warnln(fmt.Errorf("error retrieving runtimes: %w", err))
		}
		for _, cont := range containers {

			log := log.WithField("context", cont.Name())
			log.Println("deleting ...")

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

	if err := generateSSHConfig(false); err != nil {
		log.Trace("error generating ssh_config: %w", err)
	}
	return nil
}

func (c colimaApp) SSH(args ...string) error {
	ctx := context.Background()
	if !c.guest.Running(ctx) {
		return fmt.Errorf("%s not running", config.CurrentProfile().DisplayName)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error retrieving current working directory: %w", err)
	}
	// peek the current directory to see if it is mounted to prevent `cd` errors
	// with limactl ssh
	if err := func() error {
		conf, err := limautil.InstanceConfig()
		if err != nil {
			return err
		}
		pwd, err := util.CleanPath(workDir)
		if err != nil {
			return err
		}
		for _, m := range conf.MountsOrDefault() {
			location := m.MountPoint
			if location == "" {
				location = m.Location
			}
			location, err := util.CleanPath(location)
			if err != nil {
				log.Trace(err)
				continue
			}
			if strings.HasPrefix(pwd, location) {
				return nil
			}
		}
		return fmt.Errorf("not a mounted directory: %s", workDir)
	}(); err != nil {
		// the errors returned here is not critical and thereby silenced.
		// the goal is to prevent unnecessary warning message from Lima.
		log.Trace(fmt.Errorf("error checking if PWD is mounted: %w", err))

		// fallback to the user's homedir
		username, err := c.guest.User()
		if err == nil {
			workDir = "/home/" + username + ".linux"
		}
	}

	guest := lima.New(host.New())
	return guest.SSH(workDir, args...)
}

func (c colimaApp) Status(extended bool) error {
	ctx := context.Background()
	if !c.guest.Running(ctx) {
		return fmt.Errorf("%s is not running", config.CurrentProfile().DisplayName)
	}

	currentRuntime, err := c.currentRuntime(ctx)
	if err != nil {
		return err
	}

	driver := "QEMU"
	conf, _ := limautil.InstanceConfig()
	if !conf.Empty() {
		driver = conf.DriverLabel()
	}

	log.Println(config.CurrentProfile().DisplayName, "is running using", driver)
	log.Println("arch:", c.guest.Arch())
	log.Println("runtime:", currentRuntime)
	if conf.MountType != "" {
		log.Println("mountType:", conf.MountType)
	}

	// ip address
	if ipAddress := limautil.IPAddress(config.CurrentProfile().ID); ipAddress != "127.0.0.1" {
		log.Println("address:", ipAddress)
	}

	// docker socket
	if currentRuntime == docker.Name {
		log.Println("socket:", "unix://"+docker.HostSocketFile())
	}

	// kubernetes
	if k, err := c.Kubernetes(); err == nil && k.Running(ctx) {
		log.Println("kubernetes: enabled")
	}

	// additional details
	if extended {
		if inst, err := limautil.Instance(); err == nil {
			log.Println("cpu:", inst.CPU)
			log.Println("mem:", units.BytesSize(float64(inst.Memory)))
			log.Println("disk:", units.BytesSize(float64(inst.Disk)))
		}
	}

	return nil
}

func (c colimaApp) Version() error {
	ctx := context.Background()
	if !c.guest.Running(ctx) {
		return nil
	}

	containerRuntimes, err := c.currentContainerEnvironments(ctx)
	if err != nil {
		return err
	}

	var kube environment.Container
	for _, cont := range containerRuntimes {
		switch cont.Name() {
		case kubernetes.Name:
			kube = cont
			continue
		}

		fmt.Println()
		fmt.Println("runtime:", cont.Name())
		fmt.Println("arch:", c.guest.Arch())
		fmt.Println(cont.Version(ctx))
	}

	if kube != nil && kube.Version(ctx) != "" {
		fmt.Println()
		fmt.Println(kubernetes.Name)
		fmt.Println(kube.Version(ctx))
	}

	return nil
}

func (c colimaApp) currentRuntime(ctx context.Context) (string, error) {
	if !c.guest.Running(ctx) {
		return "", fmt.Errorf("%s is not running", config.CurrentProfile().DisplayName)
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

func (c colimaApp) setKubernetes(conf config.Kubernetes) error {
	b, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	return c.guest.Set(kubernetes.ConfigKey, string(b))
}

func (c colimaApp) currentContainerEnvironments(ctx context.Context) ([]environment.Container, error) {
	var containers []environment.Container

	// runtime
	{
		runtime, err := c.currentRuntime(ctx)
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
	if k, err := c.containerEnvironment(kubernetes.Name); err == nil && k.Running(ctx) {
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
	return c.currentRuntime(context.Background())
}

func (c colimaApp) Kubernetes() (environment.Container, error) {
	return c.containerEnvironment(kubernetes.Name)
}

func (c colimaApp) Active() bool {
	return c.guest.Running(context.Background())
}

func generateSSHConfig(modifySSHConfig bool) error {
	instances, err := limautil.Instances()
	if err != nil {
		return fmt.Errorf("error retrieving instances: %w", err)
	}
	var buf bytes.Buffer

	for _, i := range instances {
		if !i.Running() {
			continue
		}

		profile := config.Profile(i.Name)
		resp, err := limautil.ShowSSH(profile.ID)
		if err != nil {
			log.Trace(fmt.Errorf("error retrieving SSH config for '%s': %w", i.Name, err))
			continue
		}

		fmt.Fprintln(&buf, resp.Output)
	}

	sshFileColima := config.SSHConfigFile()
	if err := os.WriteFile(sshFileColima, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("error writing ssh_config file: %w", err)
	}

	if !modifySSHConfig {
		// ~/.ssh/config modification disabled
		return nil
	}

	includeLine := "Include " + sshFileColima

	sshFileSystem := filepath.Join(util.HomeDir(), ".ssh", "config")

	// include the SSH config file if not included
	// if ssh file missing, the only content will be the include
	if _, err := os.Stat(sshFileSystem); err != nil {
		if err := os.MkdirAll(filepath.Dir(sshFileSystem), 0700); err != nil {
			return fmt.Errorf("error creating ssh directory: %w", err)
		}

		if err := os.WriteFile(sshFileSystem, []byte(includeLine), 0644); err != nil {
			return fmt.Errorf("error modifying %s: %w", sshFileSystem, err)
		}

		return nil
	}

	sshContent, err := os.ReadFile(sshFileSystem)
	if err != nil {
		return fmt.Errorf("error reading ssh config: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(sshContent))
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())

		// empty line
		if len(words) == 0 {
			continue
		}

		// comment
		if strings.HasPrefix(words[0], "#") {
			continue
		}

		// not an include line
		if len(words) < 2 {
			continue
		}

		if words[0] == "Include" {
			sshConfig := words[1]
			sshConfig = strings.Replace(sshConfig, "~/", "$HOME/", 1)
			sshConfig = os.ExpandEnv(sshConfig)
			if sshConfig == sshFileColima {
				// already present
				return nil
			}
		}
	}

	// not found, prepend file
	if err := os.WriteFile(sshFileSystem, []byte(includeLine+"\n\n"+string(sshContent)), 0644); err != nil {
		return fmt.Errorf("error modifying %s: %w", sshFileSystem, err)
	}
	return nil
}

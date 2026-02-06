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

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/store"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/docker/go-units"
	log "github.com/sirupsen/logrus"
)

type App interface {
	Active() bool
	Start(config.Config) error
	Stop(force bool) error
	Delete(data, force bool) error
	SSH(args ...string) error
	Status(extended bool, json bool) error
	Version() error
	Runtime() (string, error)
	Update() error
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

func (c colimaApp) startWithRuntime(conf config.Config, output *terminal.Output) ([]environment.Container, error) {
	kubernetesEnabled := conf.Kubernetes.Enabled

	// Kubernetes can only be enabled for docker and containerd
	switch conf.Runtime {
	case docker.Name, containerd.Name:
	default:
		kubernetesEnabled = false
	}

	var containers []environment.Container

	{
		runtime := conf.Runtime
		if kubernetesEnabled {
			runtime += "+k3s"
		}
		if output != nil {
			output.Child("runtime " + runtime)
		} else {
			log.Println("runtime:", runtime)
		}
	}

	// runtime
	{
		env, err := c.containerEnvironment(conf.Runtime)
		if err != nil {
			return nil, err
		}
		containers = append(containers, env)
	}

	// kubernetes should come after required runtime
	if kubernetesEnabled {
		env, err := c.containerEnvironment(kubernetes.Name)
		if err != nil {
			return nil, err
		}
		containers = append(containers, env)
	}

	return containers, nil
}

func (c colimaApp) Start(conf config.Config) error {
	// create terminal output manager
	output := terminal.NewOutput()
	output.Start()
	defer output.Stop()

	ctx := context.WithValue(context.Background(), config.CtxKey(), conf)
	ctx = context.WithValue(ctx, terminal.CtxKeyOutput, output)

	// print the full path of current profile being used
	log.Tracef("starting with config file: %s\n", config.CurrentProfile().File())

	output.Begin("starting " + config.CurrentProfile().DisplayName)

	var containers []environment.Container
	if !environment.IsNoneRuntime(conf.Runtime) {
		cs, err := c.startWithRuntime(conf, output)
		if err != nil {
			output.Error(err.Error())
			return err
		}
		containers = cs
	}

	// the order for start is:
	//   vm start -> container runtime provision -> container runtime start

	// start vm
	if err := c.guest.Start(ctx, conf); err != nil {
		output.Error(err.Error())
		return fmt.Errorf("error starting vm: %w", err)
	}

	// provision and start container runtimes
	for _, cont := range containers {
		output.Begin(cont.Name())
		output.Child("provisioning")
		if err := cont.Provision(ctx); err != nil {
			output.Error(err.Error())
			return fmt.Errorf("error provisioning %s: %w", cont.Name(), err)
		}
		output.Child("starting")
		if err := cont.Start(ctx); err != nil {
			output.Error(err.Error())
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

	output.Done("done")

	if err := generateSSHConfig(conf.SSHConfig); err != nil {
		log.Trace("error generating ssh_config: %w", err)
	}
	return nil
}

func (c colimaApp) Stop(force bool) error {
	// create terminal output manager
	output := terminal.NewOutput()
	output.Start()
	defer output.Stop()

	ctx := context.Background()
	ctx = context.WithValue(ctx, terminal.CtxKeyOutput, output)

	output.Begin("stopping " + config.CurrentProfile().DisplayName)

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

			output.Begin(cont.Name())
			output.Child("stopping")

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
		output.Error(err.Error())
		return fmt.Errorf("error stopping vm: %w", err)
	}

	output.Done("done")

	if err := generateSSHConfig(false); err != nil {
		log.Trace("error generating ssh_config: %w", err)
	}
	return nil
}

func (c colimaApp) Delete(data, force bool) error {
	confirmContainerDestruction := func() bool {
		return cli.Prompt("\033[31m\033[1mthis will delete ALL container data. Are you sure you want to continue")
	}

	s, _ := store.Load()
	diskInUse := s.DiskFormatted

	if !force {
		y := cli.Prompt("are you sure you want to delete " + config.CurrentProfile().DisplayName + " and all settings")
		if !y {
			return nil
		}

		// runtime disk not in use or data deletion is requested,
		// deletion deletes all data, warn accordingly.
		if !diskInUse || data {
			if y := confirmContainerDestruction(); !y {
				return nil
			}
		}
	}

	// create terminal output manager
	output := terminal.NewOutput()
	output.Start()
	defer output.Stop()

	ctx := context.Background()
	ctx = context.WithValue(ctx, terminal.CtxKeyOutput, output)

	output.Begin("deleting " + config.CurrentProfile().DisplayName)

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
			output.Begin(cont.Name())
			output.Child("deleting")

			if err := cont.Teardown(ctx); err != nil {
				// failure here is not fatal
				log.Warnln(fmt.Errorf("error during teardown of %s: %w", cont.Name(), err))
			}
		}
	}

	// teardown vm
	output.Begin("vm")
	output.Child("deleting")
	if err := c.guest.Teardown(ctx); err != nil {
		output.Error(err.Error())
		return fmt.Errorf("error during teardown of vm: %w", err)
	}

	// delete configs
	if err := configmanager.Teardown(); err != nil {
		output.Error(err.Error())
		return fmt.Errorf("error deleting configs: %w", err)
	}

	// delete runtime disk if disk in use and data deletion is requested
	if diskInUse && data {
		output.Child("deleting container data")
		if err := limautil.DeleteDisk(); err != nil {
			output.Error(err.Error())
			return fmt.Errorf("error deleting container data: %w", err)
		}

		if err := store.Reset(); err != nil {
			log.Trace("error resetting store: %w", err)
		}
	}

	output.Done("done")

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
		conf, err := configmanager.LoadInstance()
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

type statusInfo struct {
	DisplayName      string `json:"display_name"`
	Driver           string `json:"driver"`
	Arch             string `json:"arch"`
	Runtime          string `json:"runtime"`
	MountType        string `json:"mount_type"`
	IPAddress        string `json:"ip_address,omitempty"`
	DockerSocket     string `json:"docker_socket,omitempty"`
	ContainerdSocket string `json:"containerd_socket,omitempty"`
	BuildkitdSocket  string `json:"buildkitd_socket,omitempty"`
	IncusSocket      string `json:"incus_socket,omitempty"`
	Kubernetes       bool   `json:"kubernetes"`
	CPU              int    `json:"cpu"`
	Memory           int64  `json:"memory"`
	Disk             int64  `json:"disk"`
}

func (c colimaApp) getStatus() (status statusInfo, err error) {
	ctx := context.Background()
	if !c.guest.Running(ctx) {
		return status, fmt.Errorf("%s is not running", config.CurrentProfile().DisplayName)
	}

	currentRuntime, err := c.currentRuntime(ctx)
	if err != nil {
		return status, err
	}

	status.DisplayName = config.CurrentProfile().DisplayName
	status.Driver = "QEMU"
	conf, _ := configmanager.LoadInstance()
	if !conf.Empty() {
		status.Driver = conf.DriverLabel()
	}
	status.Arch = string(c.guest.Arch())
	status.Runtime = currentRuntime
	status.MountType = conf.MountType
	ipAddress := limautil.IPAddress(config.CurrentProfile().ID)
	if ipAddress != "127.0.0.1" {
		status.IPAddress = ipAddress
	}
	if currentRuntime == docker.Name {
		status.DockerSocket = "unix://" + docker.HostSocketFile()
		status.ContainerdSocket = "unix://" + containerd.HostSocketFiles().Containerd
	}
	if currentRuntime == containerd.Name {
		status.ContainerdSocket = "unix://" + containerd.HostSocketFiles().Containerd
		status.BuildkitdSocket = "unix://" + containerd.HostSocketFiles().Buildkitd
	}
	if currentRuntime == incus.Name {
		status.IncusSocket = "unix://" + incus.HostSocketFile()
	}
	if k, err := c.Kubernetes(); err == nil && k.Running(ctx) {
		status.Kubernetes = true
	}
	if inst, err := limautil.Instance(); err == nil {
		status.CPU = inst.CPU
		status.Memory = inst.Memory
		status.Disk = inst.Disk
	}
	return status, nil
}

func (c colimaApp) Status(extended bool, jsonOutput bool) error {
	status, err := c.getStatus()
	if err != nil {
		return err
	}

	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(status); err != nil {
			return fmt.Errorf("error encoding status as json: %w", err)
		}
	} else {
		log.Println(config.CurrentProfile().DisplayName, "is running using", status.Driver)
		log.Println("arch:", status.Arch)
		log.Println("runtime:", status.Runtime)
		if status.MountType != "" {
			log.Println("mountType:", status.MountType)
		}

		// ip address
		if status.IPAddress != "" {
			log.Println("address:", status.IPAddress)
		}

		// docker socket
		if status.DockerSocket != "" {
			log.Println("docker socket:", status.DockerSocket)
		}
		if status.ContainerdSocket != "" {
			log.Println("containerd socket:", status.ContainerdSocket)
		}
		if status.BuildkitdSocket != "" {
			log.Println("buildkitd socket:", status.BuildkitdSocket)
		}
		if status.IncusSocket != "" {
			log.Println("incus socket:", status.IncusSocket)
		}

		// kubernetes
		if status.Kubernetes {
			log.Println("kubernetes: enabled")
		}

		// additional details
		if extended {
			if status.CPU > 0 {
				log.Println("cpu:", status.CPU)
			}
			if status.Memory > 0 {
				log.Println("mem:", units.BytesSize(float64(status.Memory)))
			}
			if status.Disk > 0 {
				log.Println("disk:", units.BytesSize(float64(status.Disk)))
			}
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
		if cont.Name() == kubernetes.Name {
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
	err := store.Set(func(s *store.Store) {
		// update runtime if runtime disk is in use
		if s.DiskFormatted {
			s.DiskRuntime = runtime
		}
	})

	if err != nil {
		log.Traceln(fmt.Errorf("error persisting store: %w", err))
	}

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

		if environment.IsNoneRuntime(runtime) {
			return nil, nil
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

func (c *colimaApp) Update() error {
	ctx := context.Background()
	if !c.guest.Running(ctx) {
		return fmt.Errorf("runtime cannot be updated, %s is not running", config.CurrentProfile().DisplayName)
	}

	runtime, err := c.currentRuntime(ctx)
	if err != nil {
		return err
	}

	container, err := c.containerEnvironment(runtime)
	if err != nil {
		return err
	}

	oldVersion := container.Version(ctx)

	updated, err := container.Update(ctx)
	if err != nil {
		return err
	}

	if updated {
		fmt.Println()
		fmt.Println("Previous")
		fmt.Println(oldVersion)
		fmt.Println()
		fmt.Println("Current")
		fmt.Println(container.Version(ctx))
	}

	return nil
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

		profile := config.ProfileFromName(i.Name)
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

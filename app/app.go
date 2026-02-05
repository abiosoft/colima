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
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/store"
	"github.com/abiosoft/colima/util"
	"github.com/docker/go-units"
	log "github.com/sirupsen/logrus"

	// Register Apple Container VM and runtime (on supported systems)
	_ "github.com/abiosoft/colima/environment/container/apple"
	_ "github.com/abiosoft/colima/environment/vm/apple"
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

// New creates a new app with the default Lima backend.
func New() (App, error) {
	return NewWithBackend(vm.BackendLima)
}

// NewWithBackend creates a new app with the specified VM backend.
func NewWithBackend(backend vm.Backend) (App, error) {
	var guest environment.VM
	var err error

	if backend == vm.BackendLima {
		// Use Lima directly for backward compatibility
		guest = lima.New(host.New())
	} else {
		// Use registry for other backends
		guest, err = vm.NewVM(backend, host.New())
		if err != nil {
			return nil, fmt.Errorf("error creating VM backend '%s': %w", backend, err)
		}
	}

	if err := host.IsInstalled(guest); err != nil {
		return nil, fmt.Errorf("dependency check failed for VM: %w", err)
	}

	return &colimaApp{
		guest:   guest,
		backend: backend,
	}, nil
}

type colimaApp struct {
	guest   environment.VM
	backend vm.Backend
}

func (c colimaApp) startWithRuntime(conf config.Config) ([]environment.Container, error) {
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
		log.Println("runtime:", runtime)
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
	ctx := context.WithValue(context.Background(), config.CtxKey(), conf)

	log.Println("starting", config.CurrentProfile().DisplayName)
	// print the full path of current profile being used
	log.Tracef("starting with config file: %s\n", config.CurrentProfile().File())

	var containers []environment.Container
	if !environment.IsNoneRuntime(conf.Runtime) {
		cs, err := c.startWithRuntime(conf)
		if err != nil {
			return err
		}
		containers = cs
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

	// delete runtime disk if disk in use and data deletion is requested
	if diskInUse && data {
		log.Println("deleting container data")
		if err := limautil.DeleteDisk(); err != nil {
			return fmt.Errorf("error deleting container data: %w", err)
		}

		if err := store.Reset(); err != nil {
			log.Trace("error resetting store: %w", err)
		}
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

	// For non-Lima backends, use the guest directly
	if c.backend != vm.BackendLima {
		return c.guest.SSH(workDir, args...)
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
	Backend          string `json:"backend,omitempty"`
	Arch             string `json:"arch"`
	Runtime          string `json:"runtime"`
	MountType        string `json:"mount_type,omitempty"`
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
	status.Backend = string(c.backend)

	// Set driver based on backend
	if c.backend == vm.BackendApple {
		status.Driver = "Apple Container"
	} else {
		status.Driver = "QEMU"
		conf, _ := configmanager.LoadInstance()
		if !conf.Empty() {
			status.Driver = conf.DriverLabel()
		}
		status.MountType = conf.MountType

		// Lima-specific status
		ipAddress := limautil.IPAddress(config.CurrentProfile().ID)
		if ipAddress != "127.0.0.1" {
			status.IPAddress = ipAddress
		}
		if inst, err := limautil.Instance(); err == nil {
			status.CPU = inst.CPU
			status.Memory = inst.Memory
			status.Disk = inst.Disk
		}
	}

	status.Arch = string(c.guest.Arch())
	status.Runtime = currentRuntime

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
		if status.Backend != "" && status.Backend != string(vm.BackendLima) {
			log.Println("backend:", status.Backend)
		}
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

	// Use the multi-step update flow for runtimes that require stop/restart
	if updater, ok := container.(environment.AppUpdater); ok {
		return c.updateWithAppControl(ctx, updater, container)
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

func (c *colimaApp) updateWithAppControl(ctx context.Context, updater environment.AppUpdater, container environment.Container) error {
	// check for updates
	info, err := updater.CheckUpdate(ctx)
	if err != nil {
		return fmt.Errorf("error checking for updates: %w", err)
	}

	if !info.Available {
		log.Println("already up to date")
		fmt.Println(container.Version(ctx))
		return nil
	}

	// display available updates
	fmt.Println("updates available:")
	fmt.Print(info.Description)

	if !cli.Prompt(config.CurrentProfile().DisplayName + " will be stopped to install updates (sudo password may be required). proceed") {
		return nil
	}

	oldVersion := container.Version(ctx)

	// download packages while still running
	if err := updater.DownloadUpdate(ctx); err != nil {
		return fmt.Errorf("error downloading updates: %w", err)
	}

	// stop the instance
	log.Println("stopping", config.CurrentProfile().DisplayName, "for update ...")
	if err := c.Stop(false); err != nil {
		return fmt.Errorf("error stopping for update: %w", err)
	}
	time.Sleep(time.Second * 3)

	// install updates while stopped
	if err := updater.InstallUpdate(ctx); err != nil {
		return fmt.Errorf("error installing updates: %w", err)
	}

	// restart
	log.Println("restarting", config.CurrentProfile().DisplayName, "...")
	conf, err := configmanager.Load()
	if err != nil {
		return fmt.Errorf("error loading config for restart: %w", err)
	}

	if err := c.Start(conf); err != nil {
		return fmt.Errorf("error restarting after update: %w", err)
	}

	fmt.Println()
	fmt.Println("Previous")
	fmt.Println(oldVersion)
	fmt.Println()
	fmt.Println("Current")
	fmt.Println(container.Version(ctx))

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

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/core"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [profile]",
	Short: "start Colima",
	Long: `Start Colima with the specified container runtime and optional kubernetes.

Colima can also be configured with a YAML file.
Run 'colima template' to set the default configurations or 'colima start --edit' to customize before startup.
`,
	Example: "  colima start\n" +
		"  colima start --edit\n" +
		"  colima start --foreground\n" +
		"  colima start --runtime containerd\n" +
		"  colima start --kubernetes\n" +
		"  colima start --runtime containerd --kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --arch aarch64\n" +
		"  colima start --dns 1.1.1.1 --dns 8.8.8.8\n" +
		"  colima start --dns-host example.com=1.2.3.4\n" +
		"  colima start --kubernetes --k3s-arg=\"--disable=coredns,servicelb,traefik,local-storage,metrics-server\"",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		conf := startCmdArgs.Config

		if !startCmdArgs.Flags.Edit {
			if app.Active() {
				log.Warnln("already running, ignoring")
				return nil
			}
			return start(app, conf)
		}

		// edit flag is specified
		conf, err := editConfigFile()
		if err != nil {
			return err
		}

		// validate config
		if err := configmanager.ValidateConfig(conf); err != nil {
			return fmt.Errorf("error in config file: %w", err)
		}

		if app.Active() {
			if !cli.Prompt("colima is currently running, restart to apply changes") {
				return nil
			}
			if err := app.Stop(false); err != nil {
				return fmt.Errorf("error stopping :%w", err)
			}
			// pause before startup to prevent race condition
			time.Sleep(time.Second * 3)
		}

		return start(app, conf)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// validate Lima version
		if err := core.LimaVersionSupported(); err != nil {
			return fmt.Errorf("lima compatibility error: %w", err)
		}

		// combine args and current config file(if any)
		prepareConfig(cmd)

		// validate config
		if err := configmanager.ValidateConfig(startCmdArgs.Config); err != nil {
			return fmt.Errorf("error in config: %w", err)
		}

		// persist in preparation for application start
		if startCmdArgs.Flags.SaveConfig {
			if err := configmanager.Save(startCmdArgs.Config); err != nil {
				return fmt.Errorf("error preparing config file: %w", err)
			}
		}

		return nil
	},
}

const (
	defaultCPU               = 2
	defaultMemory            = 2
	defaultDisk              = 100
	defaultKubernetesVersion = kubernetes.DefaultVersion

	defaultMountTypeQEMU = "sshfs"
	defaultMountTypeVZ   = "virtiofs"
)

var (
	defaultVMType  = "qemu"
	defaultK3sArgs = []string{"--disable=traefik"}
	envSaveConfig  = osutil.EnvVar("COLIMA_SAVE_CONFIG")
)

var startCmdArgs struct {
	config.Config

	Flags struct {
		Mounts                  []string
		LegacyKubernetes        bool // for backward compatibility
		LegacyKubernetesDisable []string
		Edit                    bool
		Editor                  string
		ActivateRuntime         bool
		DNSHosts                []string
		Foreground              bool
		SaveConfig              bool
	}
}

func init() {
	runtimes := strings.Join(environment.ContainerRuntimes(), ", ")
	defaultArch := string(environment.HostArch())
	defaultVMType = environment.DefaultVMType()

	mounts := strings.Join([]string{defaultMountTypeQEMU, "9p", "virtiofs"}, ", ")
	types := strings.Join([]string{"qemu", "vz"}, ", ")

	saveConfigDefault := true
	if envSaveConfig.Exists() {
		saveConfigDefault = envSaveConfig.Bool()
	}

	root.Cmd().AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime ("+runtimes+")")
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.ActivateRuntime, "activate", true, "set as active Docker/Kubernetes context on startup")
	startCmd.Flags().IntVarP(&startCmdArgs.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().StringVar(&startCmdArgs.CPUType, "cpu-type", "", "the CPU type, options can be checked with 'qemu-system-"+defaultArch+" -cpu help'")
	startCmd.Flags().Float32VarP(&startCmdArgs.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().StringVarP(&startCmdArgs.Arch, "arch", "a", defaultArch, "architecture (aarch64, x86_64)")
	startCmd.Flags().BoolVarP(&startCmdArgs.Flags.Foreground, "foreground", "f", false, "Keep colima in the foreground")
	startCmd.Flags().StringVar(&startCmdArgs.Hostname, "hostname", "", "custom hostname for the virtual machine")
	startCmd.Flags().StringVarP(&startCmdArgs.DiskImage, "disk-image", "i", "", "file path to a custom disk image")

	// host IP addresses
	startCmd.Flags().BoolVar(&startCmdArgs.Network.HostAddresses, "network-host-addresses", false, "support port forwarding to specific host IP addresses")

	if util.MacOS() {
		// network address
		startCmd.Flags().BoolVar(&startCmdArgs.Network.Address, "network-address", false, "assign reachable IP address to the VM")

		// vm type
		if util.MacOS13OrNewer() {
			startCmd.Flags().StringVarP(&startCmdArgs.VMType, "vm-type", "t", defaultVMType, "virtual machine type ("+types+")")
			if util.MacOS13OrNewerOnArm() {
				startCmd.Flags().BoolVar(&startCmdArgs.VZRosetta, "vz-rosetta", false, "enable Rosetta for amd64 emulation")
			}
		}

		// nested virtualization
		if util.MacOSNestedVirtualizationSupported() {
			startCmd.Flags().BoolVarP(&startCmdArgs.NestedVirtualization, "nested-virtualization", "z", false, "enable nested virtualization")
		}
	}

	// config
	startCmd.Flags().BoolVarP(&startCmdArgs.Flags.Edit, "edit", "e", false, "edit the configuration file before starting")
	startCmd.Flags().StringVar(&startCmdArgs.Flags.Editor, "editor", "", `editor to use for edit e.g. vim, nano, code (default "$EDITOR" env var)`)
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.SaveConfig, "save-config", saveConfigDefault, "persist and overwrite config file with (newly) specified flags")

	// mounts
	startCmd.Flags().StringSliceVarP(&startCmdArgs.Flags.Mounts, "mount", "V", nil, "directories to mount, suffix ':w' for writable")
	startCmd.Flags().StringVar(&startCmdArgs.MountType, "mount-type", defaultMountTypeQEMU, "volume driver for the mount ("+mounts+")")
	startCmd.Flags().BoolVar(&startCmdArgs.MountINotify, "mount-inotify", true, "propagate inotify file events to the VM")

	// ssh
	startCmd.Flags().BoolVarP(&startCmdArgs.ForwardAgent, "ssh-agent", "s", false, "forward SSH agent to the VM")
	startCmd.Flags().BoolVar(&startCmdArgs.SSHConfig, "ssh-config", true, "generate SSH config in ~/.ssh/config")
	startCmd.Flags().IntVar(&startCmdArgs.SSHPort, "ssh-port", 0, "SSH server port")

	// k8s
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes.Enabled, "kubernetes", "k", false, "start with Kubernetes")
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.LegacyKubernetes, "with-kubernetes", false, "start with Kubernetes")
	startCmd.Flags().StringVar(&startCmdArgs.Kubernetes.Version, "kubernetes-version", defaultKubernetesVersion, "must match a k3s version https://github.com/k3s-io/k3s/releases")
	startCmd.Flags().StringSliceVar(&startCmdArgs.Flags.LegacyKubernetesDisable, "kubernetes-disable", nil, "components to disable for k3s e.g. traefik,servicelb")
	startCmd.Flags().StringSliceVar(&startCmdArgs.Kubernetes.K3sArgs, "k3s-arg", defaultK3sArgs, "additional args to pass to k3s")
	startCmd.Flag("with-kubernetes").Hidden = true
	startCmd.Flag("kubernetes-disable").Hidden = true

	// env
	startCmd.Flags().StringToStringVar(&startCmdArgs.Env, "env", nil, "environment variables for the VM")

	// dns
	startCmd.Flags().IPSliceVarP(&startCmdArgs.Network.DNSResolvers, "dns", "n", nil, "DNS resolvers for the VM")
	startCmd.Flags().StringSliceVar(&startCmdArgs.Flags.DNSHosts, "dns-host", nil, "custom DNS names to provide to resolver")
}

func dnsHostsFromFlag(hosts []string) map[string]string {
	mapping := make(map[string]string)

	for _, h := range hosts {
		str := strings.SplitN(h, "=", 2)
		if len(str) != 2 {
			log.Warnf("unable to parse custom dns host: %v, skipping\n", h)
			continue
		}
		src := str[0]
		target := str[1]

		mapping[src] = target
	}
	return mapping
}

// mountsFromFlag converts mounts from cli flag format to config file format
func mountsFromFlag(mounts []string) []config.Mount {
	mnts := make([]config.Mount, len(mounts))
	for i, mount := range mounts {
		str := strings.SplitN(mount, ":", 3)
		mnt := config.Mount{Location: str[0]}

		if len(str) > 1 {
			if filepath.IsAbs(str[1]) {
				mnt.MountPoint = str[1]
			} else if str[1] == "w" {
				mnt.Writable = true
			}
		}
		if len(str) > 2 && str[2] == "w" {
			mnt.Writable = true
		}

		mnts[i] = mnt
	}
	return mnts
}

func setFlagDefaults(cmd *cobra.Command) {
	if startCmdArgs.VMType == "" {
		startCmdArgs.VMType = defaultVMType
	}

	if util.MacOS13OrNewer() {
		// changing to vz implies changing mount type to virtiofs
		if cmd.Flag("vm-type").Changed && startCmdArgs.VMType == "vz" && !cmd.Flag("mount-type").Changed {
			startCmdArgs.MountType = "virtiofs"
			cmd.Flag("mount-type").Changed = true
		}
	}

	// mount type
	{
		// convert mount type for qemu
		if startCmdArgs.VMType != "vz" && startCmdArgs.MountType == defaultMountTypeVZ {
			startCmdArgs.MountType = defaultMountTypeQEMU
			if cmd.Flag("mount-type").Changed {
				log.Warnf("%s is only available for 'vz' vmType, using %s", defaultMountTypeVZ, defaultMountTypeQEMU)
			}
		}
		// convert mount type for vz
		if startCmdArgs.VMType == "vz" && startCmdArgs.MountType == "9p" {
			startCmdArgs.MountType = "virtiofs"
			if cmd.Flag("mount-type").Changed {
				log.Warnf("9p is only available for 'qemu' vmType, using %s", defaultMountTypeVZ)
			}
		}
	}

	// always enable nested virtualization for incus, if supported and not explicitly disabled.
	if util.MacOSNestedVirtualizationSupported() {
		if !cmd.Flag("nested-virtualization").Changed {
			if startCmdArgs.Runtime == incus.Name && startCmdArgs.VMType == "vz" {
				startCmdArgs.NestedVirtualization = true
			}
		}
	}
}

func setConfigDefaults(conf *config.Config) {
	// handle macOS virtualization.framework transition
	if conf.VMType == "" {
		conf.VMType = defaultVMType
		// if on macOS with no qemu, use vz
		if err := util.AssertQemuImg(); err != nil && util.MacOS13OrNewer() {
			conf.VMType = "vz"
		}
	}

	if conf.MountType == "" {
		conf.MountType = defaultMountTypeQEMU
		if util.MacOS13OrNewer() && conf.VMType == "vz" {
			conf.MountType = defaultMountTypeVZ
		}
	}

	if conf.Hostname == "" {
		conf.Hostname = config.CurrentProfile().ID
	}
}

func setFixedConfigs(conf *config.Config) {
	fixedConf, err := configmanager.LoadFrom(config.CurrentProfile().StateFile())
	if err != nil {
		return
	}

	warnIfNotEqual := func(name, newVal, fixedVal string) {
		if newVal != fixedVal {
			log.Warnln(fmt.Errorf("'%s' cannot be updated after initial setup, discarded", name))
		}
	}

	// override the fixed configs
	// arch, vmType, mountType, runtime are fixed and cannot be changed
	if fixedConf.Arch != "" {
		warnIfNotEqual("architecture", conf.Arch, fixedConf.Arch)
		conf.Arch = fixedConf.Arch
	}
	if fixedConf.VMType != "" {
		warnIfNotEqual("virtual machine type", conf.VMType, fixedConf.VMType)
		conf.VMType = fixedConf.VMType
	}
	if fixedConf.Runtime != "" {
		warnIfNotEqual("runtime", conf.Runtime, fixedConf.Runtime)
		conf.Runtime = fixedConf.Runtime
	}
	if fixedConf.MountType != "" {
		warnIfNotEqual("volume mount type", conf.MountType, fixedConf.MountType)
		conf.MountType = fixedConf.MountType
	}
	if fixedConf.Network.Address && !conf.Network.Address {
		log.Warnln("network address cannot be disabled once enabled")
		conf.Network.Address = true
	}
}

func prepareConfig(cmd *cobra.Command) {
	current, err := configmanager.Load()
	if err != nil {
		// not fatal, will proceed with defaults
		log.Warnln(fmt.Errorf("config load failed: %w", err))
		log.Warnln("reverting to default settings")
	}

	// handle legacy kubernetes flag
	if cmd.Flag("with-kubernetes").Changed {
		startCmdArgs.Kubernetes.Enabled = startCmdArgs.Flags.LegacyKubernetes
		cmd.Flag("kubernetes").Changed = true
	}

	// convert cli to config file format
	startCmdArgs.Mounts = mountsFromFlag(startCmdArgs.Flags.Mounts)
	startCmdArgs.Network.DNSHosts = dnsHostsFromFlag(startCmdArgs.Flags.DNSHosts)
	startCmdArgs.ActivateRuntime = &startCmdArgs.Flags.ActivateRuntime

	// handle legacy kubernetes-disable
	for _, disable := range startCmdArgs.Flags.LegacyKubernetesDisable {
		startCmdArgs.Kubernetes.K3sArgs = append(startCmdArgs.Kubernetes.K3sArgs, "--disable="+disable)
	}

	// set relevant missing default values
	setFlagDefaults(cmd)

	// if there is no existing settings
	if current.Empty() {
		// attempt template
		template, err := configmanager.LoadFrom(templateFile())
		if err != nil {
			// use default config if there is no template or existing settings
			return
		}
		current = template
	}

	// set missing defaults in the current config
	setConfigDefaults(&current)

	// docker can only be set in config file
	startCmdArgs.Docker = current.Docker
	// provision scripts can only be set in config file
	startCmdArgs.Provision = current.Provision

	// use current settings for unchanged configs
	// otherwise may be reverted to their default values.
	if !cmd.Flag("arch").Changed {
		startCmdArgs.Arch = current.Arch
	}
	if !cmd.Flag("disk").Changed {
		startCmdArgs.Disk = current.Disk
	}
	if !cmd.Flag("kubernetes").Changed {
		startCmdArgs.Kubernetes.Enabled = current.Kubernetes.Enabled
	}
	if !cmd.Flag("kubernetes-version").Changed {
		startCmdArgs.Kubernetes.Version = current.Kubernetes.Version
	}
	if !cmd.Flag("k3s-arg").Changed {
		startCmdArgs.Kubernetes.K3sArgs = current.Kubernetes.K3sArgs
	}
	if !cmd.Flag("runtime").Changed {
		startCmdArgs.Runtime = current.Runtime
	}
	if !cmd.Flag("cpu").Changed {
		startCmdArgs.CPU = current.CPU
	}
	if !cmd.Flag("cpu-type").Changed {
		startCmdArgs.CPUType = current.CPUType
	}
	if !cmd.Flag("memory").Changed {
		startCmdArgs.Memory = current.Memory
	}
	if !cmd.Flag("mount").Changed {
		startCmdArgs.Mounts = current.Mounts
	}
	if !cmd.Flag("mount-type").Changed {
		startCmdArgs.MountType = current.MountType
	}
	if !cmd.Flag("mount-inotify").Changed {
		startCmdArgs.MountINotify = current.MountINotify
	}
	if !cmd.Flag("ssh-agent").Changed {
		startCmdArgs.ForwardAgent = current.ForwardAgent
	}
	if !cmd.Flag("ssh-config").Changed {
		startCmdArgs.SSHConfig = current.SSHConfig
	}
	if !cmd.Flag("ssh-port").Changed {
		startCmdArgs.SSHPort = current.SSHPort
	}
	if !cmd.Flag("dns").Changed {
		startCmdArgs.Network.DNSResolvers = current.Network.DNSResolvers
	}
	if !cmd.Flag("dns-host").Changed {
		startCmdArgs.Network.DNSHosts = current.Network.DNSHosts
	}
	if !cmd.Flag("env").Changed {
		startCmdArgs.Env = current.Env
	}
	if !cmd.Flag("hostname").Changed {
		startCmdArgs.Hostname = current.Hostname
	}
	if !cmd.Flag("activate").Changed {
		if current.ActivateRuntime != nil { // backward compatibility for `activate`
			startCmdArgs.ActivateRuntime = current.ActivateRuntime
		}
	}
	if !cmd.Flag("network-host-addresses").Changed {
		startCmdArgs.Network.HostAddresses = current.Network.HostAddresses
	}
	if util.MacOS() {
		if !cmd.Flag("network-address").Changed {
			startCmdArgs.Network.Address = current.Network.Address
		}
		if util.MacOS13OrNewer() {
			if !cmd.Flag("vm-type").Changed {
				startCmdArgs.VMType = current.VMType
			}
		}
		if util.MacOS13OrNewerOnArm() {
			if !cmd.Flag("vz-rosetta").Changed {
				startCmdArgs.VZRosetta = current.VZRosetta
			}
		}
		if util.MacOSNestedVirtualizationSupported() {
			if !cmd.Flag("nested-virtualization").Changed {
				startCmdArgs.NestedVirtualization = current.NestedVirtualization
			}
		}
	}

	setFixedConfigs(&startCmdArgs.Config)
}

// editConfigFile launches an editor to edit the config file.
func editConfigFile() (config.Config, error) {
	var c config.Config

	// preserve the current file in case the user terminates
	currentFile, err := os.ReadFile(config.CurrentProfile().File())
	if err != nil {
		return c, fmt.Errorf("error reading config file: %w", err)
	}

	// prepend the config file with termination instruction
	abort, err := embedded.ReadString("defaults/abort.yaml")
	if err != nil {
		log.Warnln(fmt.Errorf("unable to read embedded file: %w", err))
	}

	tmpFile, err := waitForUserEdit(startCmdArgs.Flags.Editor, []byte(abort+"\n"+string(currentFile)))
	if err != nil {
		return c, fmt.Errorf("error editing config file: %w", err)
	}

	// if file is empty, abort
	if tmpFile == "" {
		return c, fmt.Errorf("empty file, startup aborted")
	}

	defer func() {
		_ = os.Remove(tmpFile)
	}()
	if startCmdArgs.Flags.SaveConfig {
		if err := configmanager.SaveFromFile(tmpFile); err != nil {
			return c, err
		}
	}
	return configmanager.LoadFrom(tmpFile)
}

func start(app app.App, conf config.Config) error {
	if err := app.Start(conf); err != nil {
		return err
	}
	if startCmdArgs.Flags.Foreground {
		return awaitForInterruption(app)
	}
	return nil
}

func awaitForInterruption(app app.App) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Println("keeping Colima in the foreground, press ctrl+c to exit...")

	sig := <-c
	log.Infof("interrupted by: %v", sig)

	if err := app.Stop(false); err != nil {
		log.Errorf("error stopping: %v", err)
		return err
	}

	return nil
}

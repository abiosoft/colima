package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/util"
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
		"  colima start --runtime containerd\n" +
		"  colima start --kubernetes\n" +
		"  colima start --runtime containerd --kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --arch aarch64\n" +
		"  colima start --dns 1.1.1.1 --dns 8.8.8.8\n" +
		"  colima start --kubernetes --kubernetes-disable=coredns,servicelb,traefik,local-storage,metrics-server",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		conf := startCmdArgs.Config

		if !startCmdArgs.Flags.Edit {
			if app.Active() {
				log.Warnln("already running, ignoring")
				return nil
			}
			return app.Start(conf)
		}

		// edit flag is specified
		err := editConfigFile()
		if err != nil {
			return err
		}

		conf, err = configmanager.Load()
		if err != nil {
			return fmt.Errorf("error opening config file: %w", err)
		}

		if app.Active() {
			if !cli.Prompt("colima is currently running, restart to apply changes") {
				return nil
			}
			if err := app.Stop(false); err != nil {
				return fmt.Errorf("error stopping :%w", err)
			}
			// pause before startup to prevent race condition
			time.Sleep(time.Second * 5)
		}

		return app.Start(conf)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// combine args and current config file(if any)
		prepareConfig(cmd)

		// persist in preparing of application start
		if err := configmanager.Save(startCmdArgs.Config); err != nil {
			return fmt.Errorf("error preparing config file: %w", err)
		}

		return nil
	},
}

const (
	defaultCPU               = 2
	defaultMemory            = 2
	defaultDisk              = 60
	defaultKubernetesVersion = kubernetes.DefaultVersion
)

var defaultKubernetesDisable = []string{"traefik"}

var startCmdArgs struct {
	config.Config

	Flags struct {
		Mounts           []string
		LegacyKubernetes bool // for backward compatibility
		Edit             bool
		Editor           string
		ActivateRuntime  bool
	}
}

func init() {
	runtimes := strings.Join(environment.ContainerRuntimes(), ", ")
	defaultArch := string(environment.HostArch())

	defaultMountType := "9p"
	defaultVMTYpe := "qemu"
	if util.MacOS13() {
		defaultVMTYpe = "vz"
		defaultMountType = "virtiofs"
	}

	mounts := strings.Join([]string{defaultMountType, "sshfs"}, ", ")
	types := strings.Join([]string{defaultVMTYpe, "qemu"}, ", ")

	root.Cmd().AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime ("+runtimes+")")
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.ActivateRuntime, "activate", true, "set as active Docker/Kubernetes context on startup")
	startCmd.Flags().IntVarP(&startCmdArgs.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().StringVar(&startCmdArgs.CPUType, "cpu-type", "", "the CPU type, options can be checked with 'qemu-system-"+defaultArch+" -cpu help'")
	startCmd.Flags().IntVarP(&startCmdArgs.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().StringVarP(&startCmdArgs.Arch, "arch", "a", defaultArch, "architecture (aarch64, x86_64)")

	// network
	if util.MacOS() {
		startCmd.Flags().BoolVar(&startCmdArgs.Network.Address, "network-address", false, "assign reachable IP address to the VM")
	}
	if util.MacOS13() {
		startCmd.Flags().StringVarP(&startCmdArgs.VMType, "vm-type", "t", defaultVMTYpe, "virtual machine type ("+types+")")
	}

	// config
	startCmd.Flags().BoolVarP(&startCmdArgs.Flags.Edit, "edit", "e", false, "edit the configuration file before starting")
	startCmd.Flags().StringVar(&startCmdArgs.Flags.Editor, "editor", "", `editor to use for edit e.g. vim, nano, code (default "$EDITOR" env var)`)

	// mounts
	startCmd.Flags().StringSliceVarP(&startCmdArgs.Flags.Mounts, "mount", "V", nil, "directories to mount, suffix ':w' for writable")
	startCmd.Flags().StringVar(&startCmdArgs.MountType, "mount-type", defaultMountType, "volume driver for the mount ("+mounts+")")

	// ssh agent
	startCmd.Flags().BoolVarP(&startCmdArgs.ForwardAgent, "ssh-agent", "s", false, "forward SSH agent to the VM")

	// ssh config generation
	startCmd.Flags().BoolVar(&startCmdArgs.SSHConfig, "ssh-config", true, "generate SSH config in ~/.ssh/config")

	// k8s
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes.Enabled, "kubernetes", "k", false, "start with Kubernetes")
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.LegacyKubernetes, "with-kubernetes", false, "start with Kubernetes")
	startCmd.Flags().StringVar(&startCmdArgs.Kubernetes.Version, "kubernetes-version", defaultKubernetesVersion, "must match a k3s version https://github.com/k3s-io/k3s/releases")
	startCmd.Flags().StringSliceVar(&startCmdArgs.Kubernetes.Disable, "kubernetes-disable", defaultKubernetesDisable, "components to disable for k3s e.g. traefik,servicelb")
	startCmd.Flag("with-kubernetes").Hidden = true

	// layer
	startCmd.Flags().BoolVarP(&startCmdArgs.Layer, "layer", "l", false, "enable Ubuntu container layer")

	// env
	startCmd.Flags().StringToStringVar(&startCmdArgs.Env, "env", nil, "environment variables for the VM")

	// dns
	startCmd.Flags().IPSliceVarP(&startCmdArgs.Network.DNS, "dns", "n", nil, "DNS servers for the VM")
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
	startCmdArgs.ActivateRuntime = &startCmdArgs.Flags.ActivateRuntime

	// convert mount type for qemu
	if startCmdArgs.VMType != "vz" && startCmdArgs.MountType == "virtiofs" {
		startCmdArgs.MountType = "9p"
	}

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
	if !cmd.Flag("kubernetes-disable").Changed {
		startCmdArgs.Kubernetes.Disable = current.Kubernetes.Disable
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
	if !cmd.Flag("ssh-agent").Changed {
		startCmdArgs.ForwardAgent = current.ForwardAgent
	}
	if !cmd.Flag("ssh-config").Changed {
		startCmdArgs.SSHConfig = current.SSHConfig
	}
	if !cmd.Flag("dns").Changed {
		startCmdArgs.Network.DNS = current.Network.DNS
	}
	if !cmd.Flag("env").Changed {
		startCmdArgs.Env = current.Env
	}
	if !cmd.Flag("layer").Changed {
		startCmdArgs.Layer = current.Layer
	}
	if !cmd.Flag("activate").Changed {
		if current.ActivateRuntime != nil { // backward compatibility for `activate`
			startCmdArgs.ActivateRuntime = current.ActivateRuntime
		}
	}
	if util.MacOS() {
		if !cmd.Flag("network-address").Changed {
			startCmdArgs.Network.Address = current.Network.Address
		}
		if !cmd.Flag("vm-type").Changed {
			startCmdArgs.VMType = current.VMType
		}
	}
}

// editConfigFile launches an editor to edit the config file.
func editConfigFile() error {
	// preserve the current file in case the user terminates
	currentFile, err := os.ReadFile(config.File())
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	// prepend the config file with termination instruction
	abort, err := embedded.ReadString("defaults/abort.yaml")
	if err != nil {
		log.Warnln(fmt.Errorf("unable to read embedded file: %w", err))
	}

	tmpFile, err := waitForUserEdit(startCmdArgs.Flags.Editor, []byte(abort+"\n"+string(currentFile)))
	if err != nil {
		return fmt.Errorf("error editing config file: %w", err)
	}

	// if file is empty, abort
	if tmpFile == "" {
		return fmt.Errorf("empty file, startup aborted")
	}

	defer func() {
		_ = os.Remove(tmpFile)
	}()
	return configmanager.SaveFromFile(tmpFile)
}

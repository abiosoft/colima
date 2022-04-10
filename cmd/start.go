package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
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
	Long: `Start Colima with the specified container runtime (and kubernetes if --kubernetes is passed).
The --disk and --arch flags are only used on initial start and ignored on subsequent starts.
`,
	Example: "  colima start\n" +
		"  colima start --runtime containerd\n" +
		"  colima start --with-kubernetes\n" +
		"  colima start --runtime containerd --with-kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --arch aarch64\n" +
		"  colima start --dns 1.1.1.1 --dns 8.8.8.8",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Start(startCmdArgs.Config)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		current, err := configmanager.Load()
		if err != nil {
			// not fatal, will proceed with defaults
			log.Warnln(fmt.Errorf("config load failed: %w", err))
			log.Warnln("reverting to default settings")
		}

		// handle legacy kubernetes flag
		if cmd.Flag("with-kubernetes").Changed {
			startCmdArgs.Kubernetes.Enabled = startCmdArgs.LegacyKubernetes
			cmd.Flag("kubernetes").Changed = true
		}

		// convert cli to config file format
		startCmdArgs.Mounts = mountsFromFlag(startCmdArgs.MountsFlag)

		// use default config
		if current.Empty() {
			return nil
		}

		// disk size, and arch are only effective on VM create
		// set it to the current settings
		startCmdArgs.Disk = current.Disk
		startCmdArgs.Arch = current.Arch
		startCmdArgs.Docker = current.Docker

		// use current settings for unchanged configs
		// otherwise may be reverted to their default values.
		if !cmd.Flag("kubernetes").Changed {
			startCmdArgs.Kubernetes.Enabled = current.Kubernetes.Enabled
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
		if !cmd.Flag("dns").Changed {
			startCmdArgs.DNS = current.DNS
		}
		if util.MacOS() {
			if !cmd.Flag("network-address").Changed {
				startCmdArgs.Network.Address = current.Network.Address
			}
			if !cmd.Flag("network-user-mode").Changed {
				startCmdArgs.Network.UserMode = current.Network.UserMode
			}
		}

		log.Println("using", startCmdArgs.Runtime, "runtime")

		// remaining settings do not survive VM reboots.
		return nil
	},
	PostRunE: func(cmd *cobra.Command, args []string) error {
		return configmanager.Save(startCmdArgs.Config)
	},
}

const (
	defaultCPU               = 2
	defaultMemory            = 2
	defaultDisk              = 60
	defaultKubernetesVersion = kubernetes.DefaultVersion
)

var startCmdArgs struct {
	config.Config
	LegacyKubernetes bool `yaml:"-"` // for backward compatibility
}

func init() {
	runtimes := strings.Join(environment.ContainerRuntimes(), ", ")
	defaultArch := string(environment.Arch(runtime.GOARCH).Value())

	root.Cmd().AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime ("+runtimes+")")
	startCmd.Flags().IntVarP(&startCmdArgs.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().StringVar(&startCmdArgs.CPUType, "cpu-type", "", "the CPU type, options can be checked with 'qemu-system-"+defaultArch+" -cpu help'")
	startCmd.Flags().IntVarP(&startCmdArgs.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().StringVarP(&startCmdArgs.Arch, "arch", "a", defaultArch, "architecture (aarch64, x86_64)")

	// network
	if util.MacOS() {
		startCmd.Flags().BoolVar(&startCmdArgs.Network.Address, "network-address", true, "assign reachable IP address to the VM")
		startCmd.Flags().BoolVar(&startCmdArgs.Network.UserMode, "network-user-mode", false, "use Qemu user-mode network for internet, always true if --network-address=false")
	}

	// mounts
	startCmd.Flags().StringSliceVarP(&startCmdArgs.MountsFlag, "mount", "V", nil, "directories to mount, suffix ':w' for writable")
	startCmd.Flags().StringVar(&startCmdArgs.MountType, "mount-type", "9p", "volume driver for the mount (9p, reverse-sshfs)")

	// ssh agent
	startCmd.Flags().BoolVarP(&startCmdArgs.ForwardAgent, "ssh-agent", "s", false, "forward SSH agent to the VM")

	// k8s
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes.Enabled, "kubernetes", "k", false, "start VM with Kubernetes")
	startCmd.Flags().BoolVar(&startCmdArgs.LegacyKubernetes, "with-kubernetes", false, "start VM with Kubernetes")
	startCmd.Flags().StringVar(&startCmdArgs.Kubernetes.Version, "kubernetes-version", defaultKubernetesVersion, "must match a k3s version https://github.com/k3s-io/k3s/releases")
	startCmd.Flags().BoolVar(&startCmdArgs.Kubernetes.Ingress, "kubernetes-ingress", false, "enable traefik ingress controller")
	startCmd.Flag("with-kubernetes").Hidden = true

	// not sure of the usefulness of env vars for now considering that interactions will be with the containers, not the VM.
	// leaving it undocumented until there is a need.
	startCmd.Flags().StringToStringVarP(&startCmdArgs.Env, "env", "e", nil, "environment variables for the VM")
	_ = startCmd.Flags().MarkHidden("env")

	startCmd.Flags().IPSliceVarP(&startCmdArgs.DNS, "dns", "n", nil, "DNS servers for the VM")
}

// mountsFromFlag converts mounts from cli flag format to config file format
func mountsFromFlag(mounts []string) []config.Mount {
	mnts := make([]config.Mount, len(mounts))
	for i, mount := range mounts {
		str := strings.SplitN(string(mount), ":", 2)
		mnts[i] = config.Mount{
			Location: str[0],
			Writable: len(str) >= 2 && str[1] == "w",
		}
	}
	return mnts
}

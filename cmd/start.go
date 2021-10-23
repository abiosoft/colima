package cmd

import (
	"fmt"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net"
	"strings"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start Colima",
	Long: `Start Colima with the specified container runtime (and kubernetes if --with-kubernetes is passed).
The --runtime flag is only used on initial start and ignored on subsequent starts.
`,
	Example: "  colima start\n" +
		"  colima start --runtime containerd\n" +
		"  colima start --with-kubernetes\n" +
		"  colima start --runtime containerd --with-kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --dns 8.8.8.8 --dns 8.8.4.4\n" +
		"  colima start --mount $HOME/projects:w\n",
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Start(startCmdArgs.Config)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// set port
		startCmdArgs.VM.SSHPort = randomAvailablePort()

		current, err := config.Load()
		if err != nil {
			// not fatal, will proceed with defaults
			log.Warnln(fmt.Errorf("config load failed: %w", err))
			log.Warnln("reverting to default settings")
		}

		// use default config
		if current.Empty() {
			return nil
		}

		// runtime, ssh port, disk size and kubernetes version are only effective on VM create
		// set it to the current settings
		startCmdArgs.Runtime = current.Runtime
		startCmdArgs.VM.Disk = current.VM.Disk
		startCmdArgs.Kubernetes.Version = current.Kubernetes.Version

		// use current settings for unchanged configs
		// otherwise may be reverted to their default values.
		if !cmd.Flag("with-kubernetes").Changed {
			startCmdArgs.Kubernetes.Enabled = current.Kubernetes.Enabled
		}
		if !cmd.Flag("cpu").Changed {
			startCmdArgs.VM.CPU = current.VM.CPU
		}
		if !cmd.Flag("memory").Changed {
			startCmdArgs.VM.Memory = current.VM.Memory
		}
		if !cmd.Flag("mount").Changed {
			startCmdArgs.VM.Mounts = current.VM.Mounts
		}
		if !cmd.Flag("arch").Changed {
			startCmdArgs.VM.Arch = current.VM.Arch
		}

		log.Println("using", current.Runtime, "runtime")

		// remaining settings do not survive VM reboots.
		return nil
	},
	PostRunE: func(cmd *cobra.Command, args []string) error {
		return config.Save(startCmdArgs.Config)
	},
}

const (
	defaultCPU               = 2
	defaultMemory            = 2
	defaultDisk              = 60
	defaultArch              = "default"
	defaultKubernetesVersion = "v1.22.2"
)

var startCmdArgs struct {
	config.Config
}

func randomAvailablePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(fmt.Errorf("error picking an available port: %w", err))
	}

	if err := listener.Close(); err != nil {
		log.Fatal(fmt.Errorf("error closing temporary port listener: %w", err))
	}

	return listener.Addr().(*net.TCPAddr).Port
}

func init() {
	runtimes := strings.Join(environment.ContainerRuntimes(), ", ")

	root.Cmd().AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime, one of ["+runtimes+"]")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().IPSliceVarP(&startCmdArgs.VM.DNS, "dns", "n", nil, "DNS servers for the VM")
	startCmd.Flags().StringVarP(&startCmdArgs.VM.Arch, "arch", "a", defaultArch, "architecture (aarch64 / x86_64)")

	// mounts
	startCmd.Flags().StringSliceVarP(&startCmdArgs.VM.Mounts, "mount", "v", nil, "directories to mount, suffix ':w' for writable")

	// k8s
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes.Enabled, "with-kubernetes", "k", false, "start VM with Kubernetes")
	startCmd.Flags().StringVar(&startCmdArgs.Kubernetes.Version, "kubernetes-version", defaultKubernetesVersion, "the Kubernetes version")
	// not so familiar with k3s versioning atm, hide for now.
	_ = startCmd.Flags().MarkHidden("kubernetes-version")

	// not sure of the usefulness of env vars for now considering that interactions will be with the containers, not the VM.
	// leaving it undocumented until there is a need.
	startCmd.Flags().StringToStringVarP(&startCmdArgs.VM.Env, "env", "e", nil, "environment variables for the VM")
	_ = startCmd.Flags().MarkHidden("env")
}

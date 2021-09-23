package cmd

import (
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

// startCmd represents the start command
// TODO detect the default container runtime
// TODO replace $HOME env var.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start Colima",
	Long: `Start Colima with the specified container runtime (and kubernetes if --with-kubernetes is passed).
The --runtime flag is only used on initial start and ignored on subsequent starts.

Kubernetes requires at least 2 CPUs and 2.3GiB memory.

For verbose output, tail the log file "$HOME/Library/Caches/colima/out.log".
  tail -f "$HOME/Library/Caches/colima/out.log"
`,
	Example: "  colima start\n" +
		"  colima start --runtime containerd\n" +
		"  colima start --with-kubernetes\n" +
		"  colima start --runtime containerd --with-kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --dns 8.8.8.8 --dns 8.8.4.4\n",
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Start(startCmdArgs.Config)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		current, err := config.Load()
		if err != nil {
			// not fatal, will proceed with defaults
			log.Println(err)
			log.Println("reverting to default settings")
		}

		// use default config
		if current.Empty() {
			return nil
		}

		// runtime and disk size are only effective on VM create
		// set it to the current settings
		startCmdArgs.Runtime = current.Runtime
		startCmdArgs.VM.Disk = current.VM.Disk

		// use current settings for unchanged configs
		// otherwise may be reverted to their default values.
		if !cmd.Flag("with-kubernetes").Changed {
			startCmdArgs.Kubernetes = current.Kubernetes
		}
		if !cmd.Flag("cpu").Changed {
			startCmdArgs.VM.CPU = current.VM.CPU
		}
		if !cmd.Flag("memory").Changed {
			startCmdArgs.VM.Memory = current.VM.Memory
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
	defaultCPU     = 2
	defaultMemory  = 4
	defaultDisk    = 60
	defaultSSHPort = 41122
)

var startCmdArgs struct {
	config.Config
}

func init() {
	runtimes := strings.Join(container.Names(), ", ")

	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes, "with-kubernetes", "k", false, "start VM with Kubernetes")
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime, one of ["+runtimes+"]")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.VM.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().IPSliceVarP(&startCmdArgs.VM.DNS, "dns", "n", nil, "DNS servers for the VM")

	// internal
	startCmd.Flags().IntVar(&startCmdArgs.VM.SSHPort, "ssh-port", defaultSSHPort, "SSH port for the VM")
	startCmd.Flags().MarkHidden("ssh-port")

	// not sure of the usefulness of env vars for now considering that interactions will be with the containers, not the VM.
	// leaving it undocumented until there is a need.
	startCmd.Flags().StringToStringVarP(&startCmdArgs.VM.Env, "env", "e", nil, "environment variables for the VM")
	startCmd.Flags().MarkHidden("env")
}
package cmd

import (
	"github.com/abiosoft/colima"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/runtime/container"
	"github.com/abiosoft/colima/runtime/container/docker"
	"github.com/spf13/cobra"
	"log"
	"strings"
)

// startCmd represents the start command
// TODO detect the default container runtime
// TODO replace $HOME env var.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Colima",
	Long: `Start Colima with the specified container runtime (and kubernetes if --with-kubernetes is passed).

Kubernetes requires at least 2 CPUs and 2.3GiB memory.

For verbose output, tail the log file "$HOME/Library/Caches/colima/out.log".
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conf := cmd.Context().Value(contextKey).(*config.Config)
		app, err := colima.New(*conf)
		if err != nil {
			return err
		}

		return app.Start()
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		conf := cmd.Context().Value(contextKey).(*config.Config)

		current, err := config.Load()
		if err != nil {
			// not fatal, will proceed with defaults
			log.Println(err)
		}

		// use default config
		if current.Empty() {
			return nil
		}

		// runtime and memory are only effective on VM create
		// set it to the current settings
		conf.Runtime = current.Runtime
		conf.VM.Memory = current.VM.Memory

		// use current settings for unchanged configs
		// otherwise may be reverted to their default values.
		if !cmd.Flag("with-kubernetes").Changed {
			conf.Kubernetes = current.Kubernetes
		}
		if !cmd.Flag("cpu").Changed {
			conf.VM.CPU = current.VM.CPU
		}
		if !cmd.Flag("memory").Changed {
			conf.VM.Memory = current.VM.Memory
		}
		if !cmd.Flag("disk").Changed {
			conf.VM.Disk = current.VM.Disk
		}

		// remaining settings do not survive VM reboots.
		return nil
	},
	PostRunE: func(cmd *cobra.Command, args []string) error {
		conf := cmd.Context().Value(contextKey).(*config.Config)
		return config.Save(*conf)
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

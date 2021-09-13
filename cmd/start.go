package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
// TODO detect the default container runtime
// TODO replace $HOME env var.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start colima VM",
	Long: `Start (and/or provision) the VM with docker (and kubernetes
if --with-kubernetes is passed).

Kubernetes requires at least 2 CPUs and 2.3GiB memory.

For verbose output, tail the log file "$HOME/.colima/out.log".
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called")
	},
}

const (
	defaultCPU    = 2
	defaultMemory = 4
	defaultDisk   = 60
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolP("with-kubernetes", "k", false, "start VM with Kubernetes")
	startCmd.Flags().IntP("cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().IntP("memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntP("disk", "d", defaultDisk, "disk size in GiB")
}

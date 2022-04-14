package daemon

import (
	"github.com/abiosoft/colima/cmd/daemon/vmnet"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "daemon",
	Long:   `runner for background daemons.`,
	Hidden: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
}

func init() {
	root.Cmd().AddCommand(daemonCmd)

	daemonCmd.AddCommand(vmnet.Cmd())
}

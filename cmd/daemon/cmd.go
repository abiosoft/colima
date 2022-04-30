package daemon

import (
	"context"
	"time"

	"github.com/abiosoft/colima/environment/vm/lima/network/daemon"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/gvproxy"
	"github.com/abiosoft/colima/environment/vm/lima/network/daemon/vmnet"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
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

var startCmd = &cobra.Command{
	Use:   "start [profile]",
	Short: "start daemon",
	Long:  `start the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		var processes []daemon.Process
		if daemonArgs.vmnet {
			processes = append(processes, vmnet.New())
		}
		if daemonArgs.gvproxy {
			processes = append(processes, gvproxy.New())
		}

		return start(cmd.Context(), processes)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [profile]",
	Short: "stop daemon",
	Long:  `stop the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		// wait for 60 seconds
		timeout := time.Second * 60
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		return stop(ctx)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "status of the daemon",
	Long:  `status of the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		return status()
	},
}

var daemonArgs struct {
	vmnet   bool
	gvproxy bool
}

func init() {
	root.Cmd().AddCommand(daemonCmd)

	daemonCmd.AddCommand(startCmd)
	daemonCmd.AddCommand(stopCmd)
	daemonCmd.AddCommand(statusCmd)

	startCmd.Flags().BoolVar(&daemonArgs.vmnet, "vmnet", false, "start vmnet")
	startCmd.Flags().BoolVar(&daemonArgs.gvproxy, "gvproxy", false, "start gvproxy")
}

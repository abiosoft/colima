package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/spf13/cobra"
)

// autostartCmd represents the autostart command
var autostartCmd = &cobra.Command{
	Use:   "autostart",
	Short: "manage automatic startup",
	Long:  `Manage automatic startup of Colima.`,
}

var autostartCmdArgs struct {
	boot bool
}

var autostartEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "enable automatic startup",
	Long: `Enable automatic startup of Colima.

By default the instance is started when the user logs in. With --boot it is
started at system boot instead, before any user logs in, which is what a
headless machine needs. That installs a system LaunchDaemon and therefore
requires sudo.

Use --profile to target an instance other than the default.

This registers the instance with Lima, so the unit runs "limactl start" rather
than "colima start". Provision scripts configured with mode "afterBoot" or
"ready" therefore do not run on an automatic start.

Requires Lima v2.3.0 or newer.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		condition := limautil.AutostartLogin
		if autostartCmdArgs.boot {
			condition = limautil.AutostartBoot
		}
		return limautil.EnableAutostart(condition)
	},
}

var autostartDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "disable automatic startup",
	Long:  `Disable automatic startup of Colima.`,
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return limautil.DisableAutostart()
	},
}

func init() {
	root.Cmd().AddCommand(autostartCmd)
	autostartCmd.AddCommand(autostartEnableCmd)
	autostartCmd.AddCommand(autostartDisableCmd)

	autostartEnableCmd.Flags().BoolVar(&autostartCmdArgs.boot, "boot", false,
		"start at system boot rather than at login (macOS only, requires sudo)")
}

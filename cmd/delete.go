package cmd

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/spf13/cobra"
)

var deleteCmdArgs struct {
	force bool
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [profile]",
	Short: "delete and teardown Colima",
	Long: `Delete and teardown Colima and all settings.

Use with caution. This deletes everything and a startup afterwards is like the
initial startup of Colima.

If you simply want to reset the Kubernetes cluster, run 'colima kubernetes reset'.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !deleteCmdArgs.force {
			y := cli.Prompt("are you sure you want to delete " + config.CurrentProfile().DisplayName + " and all settings")
			if !y {
				return nil
			}
			yy := cli.Prompt("\033[31m\033[1mthis will delete ALL container data. Are you sure you want to continue")
			if !yy {
				return nil
			}
		}

		return newApp().Delete()
	},
}

func init() {
	root.Cmd().AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVarP(&deleteCmdArgs.force, "force", "f", false, "do not prompt for yes/no")
}

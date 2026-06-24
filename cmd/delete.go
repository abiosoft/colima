package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

var deleteCmdArgs struct {
	force bool
	data  bool
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [profile]",
	Short: "delete and teardown Colima",
	Long: `Delete and teardown Colima and all settings.

Use with caution. This deletes everything and a startup afterwards is like the
initial startup of Colima.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Delete(deleteCmdArgs.data, deleteCmdArgs.force)
	},
}

func init() {
	root.Cmd().AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVarP(&deleteCmdArgs.force, "force", "f", false, "do not prompt for yes/no")
	deleteCmd.Flags().BoolVarP(&deleteCmdArgs.data, "data", "d", false, "delete container runtime data")
}

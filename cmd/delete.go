package cmd

import (
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete and teardown Colima",
	Long: `Delete and teardown Colima and all settings.

Use with caution. This deletes everything and a startup afterwards is like the
initial startup of Colima.

If you simply want to reset the Kubernetes cluster, run 'colima kubernetes reset'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Delete()
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

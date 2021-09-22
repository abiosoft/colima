package cmd

import (
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Colima",
	Long: `Stop stops Colima to free up resources.

The state of the VM is persisted at stop. A start afterwards
should return it back to its previous state.`,
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(app.Stop())
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

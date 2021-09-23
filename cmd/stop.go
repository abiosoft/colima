package cmd

import (
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop Colima",
	Long: `Stop stops Colima to free up resources.

The state of the VM is persisted at stop. A start afterwards
should return it back to its previous state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Stop()
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

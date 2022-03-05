package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop [profile]",
	Short: "stop Colima",
	Long: `Stop stops Colima to free up resources.

The state of the VM is persisted at stop. A start afterwards
should return it back to its previous state.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Stop()
	},
}

func init() {
	root.Cmd().AddCommand(stopCmd)
}

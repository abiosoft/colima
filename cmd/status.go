package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [profile]",
	Short: "show the status of Colima",
	Long:  `Show the status of Colima`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Status()
	},
}

func init() {
	root.Cmd().AddCommand(statusCmd)
}

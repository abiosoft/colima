package cmd

import (
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "show the status of Colima",
	Long:  `Show the status of Colima`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Status()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

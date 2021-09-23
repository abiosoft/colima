package cmd

import (
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the version of Colima",
	Long:  `Print the version of Colima`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Version()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
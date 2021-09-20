package cmd

import (
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Colima",
	Long:  `Print the version of Colima`,
	Run: func(cmd *cobra.Command, args []string) {
		cobra.CheckErr(app.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

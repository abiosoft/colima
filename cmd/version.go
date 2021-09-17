package cmd

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Colima",
	Long:  `Print the version of Colima`,
	Run: func(cmd *cobra.Command, args []string) {
		name := config.AppName()
		version := config.AppVersion()
		fmt.Println(name, "version", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

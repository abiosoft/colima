package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version [profile]",
	Short: "print the version of Colima",
	Long:  `Print the version of Colima`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := config.AppVersion()
		fmt.Println(config.AppName, "version", version.Version)
		fmt.Println("git commit:", version.Revision)

		if colimaApp, err := app.New(); err == nil {
			_ = colimaApp.Version()
		}
	},
}

func init() {
	root.Cmd().AddCommand(versionCmd)
}

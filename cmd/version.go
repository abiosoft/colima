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
	Use:   "version",
	Short: "print the version of Colima",
	Long:  `Print the version of Colima`,
	Run: func(cmd *cobra.Command, args []string) {
		name := config.Profile()
		version := config.AppVersion()
		fmt.Println(name, "version", version.Version)
		fmt.Println("git commit:", version.Revision)

		if colimaApp, err := app.New(); err == nil {
			_ = colimaApp.Version()
		}
	},
}

func init() {
	root.Cmd().AddCommand(versionCmd)
}

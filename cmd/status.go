package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

var statusCmdArgs struct {
	extended bool
	json     bool
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [profile]",
	Short: "show the status of Colima",
	Long:  `Show the status of Colima`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusCmdArgs.json {
			return newApp().Status(statusCmdArgs.extended, true)
		}
		return newApp().Status(statusCmdArgs.extended, false)
	},
}

func init() {
	root.Cmd().AddCommand(statusCmd)

	statusCmd.Flags().BoolVarP(&statusCmdArgs.extended, "extended", "e", false, "include additional details")
	statusCmd.Flags().BoolVarP(&statusCmdArgs.json, "json", "j", false, "print json output")
}

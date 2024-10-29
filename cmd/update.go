package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var updateCmd = &cobra.Command{
	Use:     "update [profile]",
	Aliases: []string{"u", "up"},
	Short:   "update the container runtime",
	Long:    `Update the current container runtime.`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newApp().Update()
	},
}

func init() {
	root.Cmd().AddCommand(updateCmd)
}

package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/abiosoft/colima/environment/vm/apple/appleutil"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:     "update [profile]",
	Aliases: []string{"u", "up"},
	Short:   "update the container runtime",
	Long:    `Update the current container runtime.`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if appleutil.AppleBackend() {
			return newAppWithBackend(vm.BackendApple).Update()
		}
		return newApp().Update()
	},
}

func init() {
	root.Cmd().AddCommand(updateCmd)
}

package cmd

import (
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/spf13/cobra"
)

var restartCmdArgs struct {
	force bool
}

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart [profile]",
	Short: "restart Colima",
	Long: `Stop and then starts Colima.

The state of the VM is persisted at stop. A start afterwards
should return it back to its previous state.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()

		stopErr := app.Stop(restartCmdArgs.force)
		if stopErr != nil {
			return stopErr
		}

		config, err := configmanager.Load()
		if err != nil {
			return err
		}

		return app.Start(config)
	},
}

func init() {
	root.Cmd().AddCommand(restartCmd)

	restartCmd.Flags().BoolVarP(&restartCmdArgs.force, "force", "f", false, "during restart, do stop without graceful shutdown")
}

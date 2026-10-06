package cmd

import (
	"fmt"
	"time"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
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
		// validate if the instance was previously created
		if _, err := limautil.Instance(); err != nil {
			return err
		}

		conf, err := restartConfig()
		if err != nil {
			return err
		}
		if err := configmanager.ValidateConfig(conf); err != nil {
			return fmt.Errorf("error in config: %w", err)
		}

		app := newApp()

		if err := app.Stop(restartCmdArgs.force); err != nil {
			return err
		}

		// delay a bit before starting
		time.Sleep(time.Second * 3)

		return app.Start(conf)
	},
}

func restartConfig() (config.Config, error) {
	current, err := configmanager.Load()
	if err != nil {
		return config.Config{}, err
	}
	// prepareConfig would ignore it and use plain defaults, which can grow the data disk
	if current.Empty() {
		return configmanager.LoadInstance()
	}
	prepareConfig(startCmd)
	return startCmdArgs.Config, nil
}

func init() {
	root.Cmd().AddCommand(restartCmd)

	restartCmd.Flags().BoolVarP(&restartCmdArgs.force, "force", "f", false, "during restart, do stop without graceful shutdown")
}

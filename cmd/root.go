package cmd

import (
	"github.com/abiosoft/colima"
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/log"
	"github.com/spf13/cobra"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   config.AppName(),
	Short: "container runtimes on macOS with minimal setup",
	Long:  `Colima provides container runtimes on macOS with minimal setup.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := initLog(rootCmdArgs.DryRun); err != nil {
			return err
		}

		cmd.SilenceUsage = true
		return nil
	},
}

var rootCmdArgs struct {
	DryRun bool
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// because Cobra is somehow too smart or too dumb to exit with error code when there is an error.
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootCmdArgs.DryRun, "dry-run", rootCmdArgs.DryRun, "perform a dry run instead")

	// decide if this should be public
	rootCmd.PersistentFlags().MarkHidden("dry-run")

}

func initLog(dryRun bool) error {
	// general log
	log.OverrideDefaultLog()

	// command logs
	out, err := os.OpenFile(config.LogFile(), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if dryRun {
		cli.DryRun(dryRun)
	}
	cli.Stdout(out)
	cli.Stderr(out)

	return nil
}

func newApp() colima.App {
	app, err := colima.New()
	cobra.CheckErr(err)
	return app
}

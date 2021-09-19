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
	Short: "Docker (and Kubernetes) on macOS with minimal setup",
	Long:  `Colima provides Docker (and Kubernetes) on macOS with minimal setup.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", dryRun, "perform a dry run instead")

	// decide if this should be public
	//rootCmd.PersistentFlags().MarkHidden("dry-run")

	cobra.OnInitialize(
		initLog,
		initApp,
	)
}

var dryRun bool
var app colima.App

func initLog() {
	// general log
	log.OverrideDefaultLog()

	// command logs
	out, err := os.OpenFile(config.LogFile(), os.O_CREATE|os.O_RDWR, 0644)
	cobra.CheckErr(err)

	if dryRun {
		cli.DryRun(dryRun)
	}
	cli.Stdout(out)
	cli.Stderr(out)

}

func initApp() {
	var err error
	app, err = colima.New(appConfig)
	cobra.CheckErr(err)
}

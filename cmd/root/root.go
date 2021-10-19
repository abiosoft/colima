package root

import (
	"log"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "colima",
	Short: "container runtimes on macOS with minimal setup",
	Long:  `Colima provides container runtimes on macOS with minimal setup.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if rootArgs.Profile != config.AppName && rootArgs.Profile != "" {
			// if custom profile is specified,
			// use a prefix to prevent possible name clashes
			config.SetProfile(config.AppName + "-" + rootArgs.Profile)
		}
		if err := initLog(rootArgs.DryRun); err != nil {
			return err
		}

		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return nil
	},
}

// Cmd returns the root command.
func Cmd() *cobra.Command {
	return rootCmd
}

// RootCmdArgs returns the currently set rootArgs of the command
func RootCmdArgs() rootCmdArgs {
	return rootArgs
}

type rootCmdArgs struct {
	DryRun  bool
	Profile string
	Verbose bool
}

var rootArgs rootCmdArgs

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootArgs.DryRun, "dry-run", rootArgs.DryRun, "perform a dry run instead")
	rootCmd.PersistentFlags().BoolVar(&rootArgs.Verbose, "verbose", rootArgs.Verbose, "verbose terminal output")
	rootCmd.PersistentFlags().StringVarP(&rootArgs.Profile, "profile", "p", config.AppName, "use different profile")

	// decide if these should be public
	// implementations are currently half-baked, only for test during development
	_ = rootCmd.PersistentFlags().MarkHidden("dry-run")
	_ = rootCmd.PersistentFlags().MarkHidden("profile")

}

func initLog(dryRun bool) error {
	// general log output
	log.SetOutput(logrus.New().Writer())
	log.SetFlags(0)

	if dryRun {
		cli.DryRun(dryRun)
	}

	return nil
}

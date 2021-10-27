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
		if rootCmdArgs.Profile != "" {
			config.SetProfile(rootCmdArgs.Profile)
		}
		if err := initLog(); err != nil {
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

// rootCmdArgs holds all flags configured in root Cmd
var rootCmdArgs struct {
	DryRun  bool
	Profile string
	Verbose bool
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootCmdArgs.DryRun, "dry-run", rootCmdArgs.DryRun, "perform a dry run instead")
	rootCmd.PersistentFlags().BoolVar(&rootCmdArgs.Verbose, "verbose", rootCmdArgs.Verbose, "enable verbose log")
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Profile, "profile", "p", config.AppName, "profile name, for multiple instances")

	// decide if these should be public
	// implementations are currently half-baked, only for test during development
	_ = rootCmd.PersistentFlags().MarkHidden("dry-run")

}

func initLog() error {
	// general log output
	log.SetOutput(logrus.New().Writer())
	log.SetFlags(0)

	if rootCmdArgs.DryRun {
		cli.DryRun(rootCmdArgs.DryRun)
	}
	if rootCmdArgs.Verbose {
		cli.Settings.Verbose = rootCmdArgs.Verbose
	}

	return nil
}

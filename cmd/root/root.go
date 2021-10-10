package root

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"log"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "colima",
	Short: "container runtimes on macOS with minimal setup",
	Long:  `Colima provides container runtimes on macOS with minimal setup.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if rootCmdArgs.Profile != config.AppName && rootCmdArgs.Profile != "" {
			// if custom profile is specified,
			// use a prefix to prevent possible name clashes
			config.SetProfile(config.AppName + "-" + rootCmdArgs.Profile)
		}
		if err := initLog(rootCmdArgs.DryRun); err != nil {
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

var rootCmdArgs struct {
	DryRun  bool
	Profile string
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
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Profile, "profile", "p", config.AppName, "use different profile")

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

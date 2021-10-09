package root

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/lineprefix"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
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
	DryRun     bool
	Profile    string
	VerboseLog bool
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
	rootCmd.PersistentFlags().BoolVarP(&rootCmdArgs.VerboseLog, "verbose", "v", rootCmdArgs.VerboseLog, "enable verbose output")

	// decide if these should be public
	// implementations are currently half-baked, only for test during development
	_ = rootCmd.PersistentFlags().MarkHidden("dry-run")
	_ = rootCmd.PersistentFlags().MarkHidden("profile")

}

func initLog(dryRun bool) error {
	logger := util.Logger()

	if rootCmdArgs.VerboseLog {
		logger.SetLevel(logrus.DebugLevel)
	}

	// general log output
	{
		log.SetOutput(logger.Writer())
		log.SetFlags(0)
	}

	// verbose output
	{
		var out io.WriteCloser = logger.WriterLevel(logrus.DebugLevel)
		out = lineprefix.New(
			lineprefix.Writer(out),
			lineprefix.Color(color.New(color.FgHiBlack)),
		)

		if dryRun {
			cli.DryRun(dryRun)
		}
		cli.Stdout(out)
		cli.Stderr(out)
	}

	return nil
}

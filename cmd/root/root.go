package root

import (
	"log"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var versionInfo = config.AppVersion()

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "colima",
	Short:   "container runtimes on macOS with minimal setup",
	Long:    `Colima provides container runtimes on macOS with minimal setup.`,
	Version: versionInfo.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		switch cmd.Name() {

		// special case handling for commands directly interacting with the VM
		// start, stop, restart, delete, status, version, update, ssh-config
		case "start",
			"stop",
			"restart",
			"delete",
			"status",
			"version",
			"update",
			"ssh-config":

			// if an arg is passed, assume it to be the profile (provided --profile is unset)
			// i.e. colima start docker == colima start --profile=docker
			if len(args) > 0 && !cmd.Flag("profile").Changed {
				rootCmdArgs.Profile = args[0]
			}
		}
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
	Profile     string
	Verbose     bool
	VeryVerbose bool
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&rootCmdArgs.Verbose, "verbose", "v", rootCmdArgs.Verbose, "enable verbose log")
	rootCmd.PersistentFlags().BoolVar(&rootCmdArgs.VeryVerbose, "very-verbose", rootCmdArgs.VeryVerbose, "enable more verbose log")
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Profile, "profile", "p", "default", "profile name, for multiple instances")
}

func initLog() error {
	if rootCmdArgs.Verbose {
		cli.Settings.Verbose = true
		logrus.SetLevel(logrus.DebugLevel)
	}
	if rootCmdArgs.VeryVerbose {
		cli.Settings.Verbose = true
		logrus.SetLevel(logrus.TraceLevel)
	}

	// general log output
	log.SetOutput(logrus.StandardLogger().Writer())
	log.SetFlags(0)

	return nil
}

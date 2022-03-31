package vmnet

import (
	"os"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/network"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := vmnetCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

// vmnetCmd represents the base command when called without any subcommands
var vmnetCmd = &cobra.Command{
	Use:   "vmnet",
	Short: "vde_vmnet runner",
	Long:  `vde_vmnet runner for vde_vmnet daemons.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
}

// startCmd represents the kubernetes start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start daemon",
	Long:  `start the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		ptp, err := network.PTPFile()
		if err != nil {
			logrus.Warnln("ptp file error: %w", err) // this should never happen
		}
		pid := strings.TrimSuffix(ptp, ".ptp") + ".pid"

		// delete existing sockets if exist
		// errors ignored on purpose
		_ = forceDeleteFileIfExists(ptp)
		_ = forceDeleteFileIfExists(ptp + "+") // created by running qemu instance

		command := cli.CommandInteractive(network.VmnetBinary,
			"--vmnet-mode", "shared",
			"--vmnet-gateway", network.VmnetGateway,
			"--vmnet-dhcp-end", "192.168.106.254",
			"--pidfile", pid,
			ptp+"[]",
		)

		return command.Run()
	},
}

func forceDeleteFileIfExists(name string) error {
	if stat, err := os.Stat(name); err == nil && !stat.IsDir() {
		return os.Remove(name)
	}
	return nil
}

func init() {
	vmnetCmd.AddCommand(startCmd)
}

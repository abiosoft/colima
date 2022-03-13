package vmnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
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

const vmnetFileName = "vmnet"
const vmnetBinary = "/opt/colima/bin/vde_vmnet"

// PTPFile returns path to the ptp socket file.
func PTPFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return dir, err
	}

	return filepath.Join(dir, vmnetFileName+".ptp"), nil
}

// Dir is the network configuration directory.
func Dir() (string, error) {
	dir := filepath.Join(config.Dir(), "network")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating network directory: %w", err)
	}
	return dir, nil
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

		ptp, err := PTPFile()
		if err != nil {
			logrus.Warnln("ptp file error: %w", err) // this should never happen
		}
		pid := strings.TrimSuffix(ptp, ".ptp") + ".pid"

		// delete existing sockets if exist
		// errors ignored on purpose
		_ = forceDeleteFileIfExists(ptp)
		_ = forceDeleteFileIfExists(ptp + "+") // created by running qemu instance

		command := cli.CommandInteractive(vmnetBinary,
			"--vmnet-mode", "shared",
			"--vmnet-gateway", "192.168.106.1",
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

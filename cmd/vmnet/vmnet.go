package vmnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/network"
	"github.com/sevlyar/go-daemon"
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

		child, err := daemonize()
		if err != nil {
			return err
		}
		if !child {
			return nil
		}

		ptp := network.PTPFile()
		pid := strings.TrimSuffix(ptp, ".ptp") + ".pid"

		// delete existing sockets if exist
		// errors ignored on purpose
		_ = forceDeleteFileIfExists(ptp)
		_ = forceDeleteFileIfExists(ptp + "+") // created by running qemu instance

		command := cli.CommandInteractive(network.VmnetBinary,
			"--vmnet-mode", "shared",
			"--vmnet-gateway", network.VmnetGateway,
			"--vmnet-dhcp-end", network.VmnetDHCPEnd,
			"--pidfile", pid,
			ptp+"[]",
		)

		return command.Run()
	},
}

// stopCmd represents the kubernetes start command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop daemon",
	Long:  `stop the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		info := info()

		// ideally, killing vmnet should kill the deamon, but just to ensure
		for _, pidFile := range []string{info.VmnetPidFile, info.PidFile} {

			if _, err := os.Stat(pidFile); err != nil {
				// there's no pidfile, process already dead
				continue
			}

			if err := cli.CommandInteractive("pkill", "-F", pidFile).Run(); err != nil {
				logrus.Error(fmt.Errorf("error killing process: %w", err))
			}
		}

		// in the rarest of cases that the pidfiles got deleted and the process is still active,
		// manually killing the process will do. ugly but works
		// TODO remove this if there is adverse effect
		filter := fmt.Sprintf(`\-\-pidfile %s`, info.VmnetPidFile)
		if err := cli.CommandInteractive("sh", "-c", `ps ax | grep "`+filter+`"`).Run(); err == nil {
			// process found
			return cli.CommandInteractive("pkill", "-f", filter).Run()
		}

		return nil
	},
}

func forceDeleteFileIfExists(name string) error {
	if stat, err := os.Stat(name); err == nil && !stat.IsDir() {
		return os.Remove(name)
	}
	return nil
}

type daemonInfo struct {
	PidFile      string
	LogFile      string
	VmnetPidFile string
}

func info() (d daemonInfo) {
	dir := network.Dir()

	d.PidFile = filepath.Join(dir, "colima-daemon.pid")
	d.LogFile = filepath.Join(dir, "vmnet.stderr")
	d.VmnetPidFile = filepath.Join(dir, "vmnet.pid")
	return d
}

// daemonize creates the deamon and returns if this is a child process
// To terminate the daemon use:
//  kill `cat sample.pid`
func daemonize() (child bool, err error) {
	info := info()
	ctx := &daemon.Context{
		PidFileName: info.PidFile,
		PidFilePerm: 0644,
		LogFileName: info.LogFile,
		LogFilePerm: 0644,
	}

	d, err := ctx.Reborn()
	if err != nil {
		return false, fmt.Errorf("error running colima-vmnet as daemon: %w", err)
	}
	if d != nil {
		return false, nil
	}
	defer func() {
		_ = ctx.Release()
	}()

	logrus.Info("- - - - - - - - - - - - - - -")
	logrus.Info("colima-vmnet daemon started")
	logrus.Infof("Run `sudo pkill -F %s` to kill the daemon", info.PidFile)

	return true, nil
}

func init() {
	vmnetCmd.AddCommand(startCmd)
	vmnetCmd.AddCommand(stopCmd)
}

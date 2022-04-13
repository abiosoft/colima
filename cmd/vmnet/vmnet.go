package vmnet

import (
	"fmt"
	"os"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/network"
	"github.com/sevlyar/go-daemon"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var vmnetCmd = &cobra.Command{
	Use:    "vmnet",
	Short:  "vde_vmnet runner",
	Long:   `vde_vmnet runner for vde_vmnet daemons.`,
	Hidden: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start daemon",
	Long:  `start the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config.SetProfile(args[0])

		ctx, child, err := daemonize()
		if err != nil {
			return err
		}

		if ctx != nil {
			defer func() {
				_ = ctx.Release()
			}()
		}

		if !child {
			return nil
		}

		vmnet := network.Info().Vmnet
		ptp := vmnet.PTPFile
		pid := vmnet.PidFile

		// delete existing sockets if exist
		// errors ignored on purpose
		_ = forceDeleteFileIfExists(ptp)
		_ = forceDeleteFileIfExists(ptp + "+") // created by running qemu instance

		// rootfully start the vmnet daemon
		command := cli.CommandInteractive("sudo", network.VmnetBinary,
			"--vmnet-mode", "shared",
			"--vde-group", "staff",
			"--vmnet-gateway", network.VmnetGateway,
			"--vmnet-dhcp-end", network.VmnetDHCPEnd,
			"--pidfile", pid,
			ptp+"[]",
		)

		return command.Run()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop daemon",
	Long:  `stop the daemon`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := args[0]
		config.SetProfile(profile)

		info := network.Info()

		// rootfully kill the vmnet process
		// process is only assumed alive if the pidfile exists
		pid := info.Vmnet.PidFile
		if _, err := os.Stat(pid); err == nil {
			if err := cli.CommandInteractive("sudo", "pkill", "-F", pid).Run(); err != nil {
				return fmt.Errorf("error killing process: %w", err)
			}
		}

		// wait some seconds for the pidfile to get deleted
		{
			const wait = time.Second * 1
			const tries = 30
			for i := 0; i < tries; i++ {
				if _, err := os.Stat(pid); err != nil {
					break
				}
				time.Sleep(wait)
			}
		}

		// kill the colima process. ideally should not exist as the blocking child process is dead.
		// process is only assumed alive if the pidfile exists
		if _, err := os.Stat(info.PidFile); err == nil {
			if err := cli.CommandInteractive("pkill", "-F", info.PidFile).Run(); err != nil {
				logrus.Error(fmt.Errorf("error killing process: %w", err))
			}
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

// daemonize creates the deamon and returns if this is a child process
// To terminate the daemon use:
//  kill `cat sample.pid`
func daemonize() (ctx *daemon.Context, child bool, err error) {
	info := network.Info()
	ctx = &daemon.Context{
		PidFileName: info.PidFile,
		PidFilePerm: 0644,
		LogFileName: info.LogFile,
		LogFilePerm: 0644,
	}

	d, err := ctx.Reborn()
	if err != nil {
		return ctx, false, fmt.Errorf("error running colima-vmnet as daemon: %w", err)
	}
	if d != nil {
		return ctx, false, nil
	}

	logrus.Info("- - - - - - - - - - - - - - -")
	logrus.Info("vmnet daemon started by colima")
	logrus.Infof("Run `pkill -F %s` to kill the daemon", info.PidFile)

	return ctx, true, nil
}

func init() {
	root.Cmd().AddCommand(vmnetCmd)

	vmnetCmd.AddCommand(startCmd)
	vmnetCmd.AddCommand(stopCmd)
}

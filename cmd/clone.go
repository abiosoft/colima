package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var cloneCmd = &cobra.Command{
	Use:   "clone <profile> <new-profile>",
	Short: "clone Colima profile",
	Long:  `Clone the Colima profile.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		from := config.ProfileFromName(args[0])
		to := config.ProfileFromName(args[1])

		logrus.Infof("preparing to clone %s...", from.DisplayName)
		{
			// verify source profile exists
			if stat, err := os.Stat(from.LimaInstanceDir()); err != nil || !stat.IsDir() {
				return fmt.Errorf("colima profile '%s' does not exist", from.ShortName)
			}

			// verify destination profile does not exists
			if stat, err := os.Stat(to.LimaInstanceDir()); err == nil && stat.IsDir() {
				return fmt.Errorf("colima profile '%s' already exists, delete with `colima delete %s` and try again", to.ShortName, to.ShortName)
			}

			// copy source to destination
			logrus.Info("cloning virtual machine...")
			if err := cli.Command("mkdir", "-p", to.LimaInstanceDir()).Run(); err != nil {
				return fmt.Errorf("error preparing to copy VM: %w", err)
			}

			if err := cli.Command("cp",
				filepath.Join(from.LimaInstanceDir(), "basedisk"),
				filepath.Join(from.LimaInstanceDir(), "diffdisk"),
				filepath.Join(from.LimaInstanceDir(), "cidata.iso"),
				filepath.Join(from.LimaInstanceDir(), "lima.yaml"),
				to.LimaInstanceDir(),
			).Run(); err != nil {
				return fmt.Errorf("error copying VM: %w", err)
			}
		}

		{
			logrus.Info("copying config...")
			// verify source config exists
			if _, err := os.Stat(from.LimaInstanceDir()); err != nil {
				return fmt.Errorf("config missing for colima profile '%s': %w", from.ShortName, err)
			}

			// ensure destination config directory
			if err := cli.Command("mkdir", "-p", filepath.Dir(to.LimaInstanceDir())).Run(); err != nil {
				return fmt.Errorf("cannot copy config to new profile '%s': %w", to.ShortName, err)
			}

			if err := cli.Command("cp", from.LimaInstanceDir(), to.LimaInstanceDir()).Run(); err != nil {
				return fmt.Errorf("error copying VM config: %w", err)
			}
		}

		logrus.Info("clone successful")
		logrus.Infof("run `colima start %s` to start the newly cloned profile", to.ShortName)
		return nil
	},
}

func init() {
	root.Cmd().AddCommand(cloneCmd)
	cloneCmd.Hidden = true

}

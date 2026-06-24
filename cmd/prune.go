package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var pruneCmdArgs struct {
	force bool
	all   bool
}

// pruneCmd represents the prune command
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "prune cached downloaded assets",
	Long:  `Prune cached downloaded assets`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		colimaCacheDir := config.CacheDir()
		limaCacheDir := filepath.Join(filepath.Dir(colimaCacheDir), "lima")
		if !pruneCmdArgs.force {
			msg := "'" + colimaCacheDir + "' will be emptied, are you sure"
			if pruneCmdArgs.all {
				msg = "'" + colimaCacheDir + "' and '" + limaCacheDir + "' will be emptied, are you sure"
			}
			if y := cli.Prompt(msg); !y {
				return nil
			}
		}
		logrus.Info("Pruning ", strconv.Quote(config.CacheDir()))
		if err := os.RemoveAll(config.CacheDir()); err != nil {
			return fmt.Errorf("error during prune: %w", err)
		}

		if pruneCmdArgs.all {
			cmd := limautil.Limactl("prune")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("error during Lima prune: %w", err)
			}
		}

		return nil
	},
}

func init() {
	root.Cmd().AddCommand(pruneCmd)

	pruneCmd.Flags().BoolVarP(&pruneCmdArgs.force, "force", "f", false, "do not prompt for yes/no")
	pruneCmd.Flags().BoolVarP(&pruneCmdArgs.all, "all", "a", false, "include Lima assets")
}

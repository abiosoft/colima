package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var sshConfigCmd = &cobra.Command{
	Use:   "ssh-config [profile]",
	Short: "show SSH connection config",
	Long:  `Show configuration of the SSH connection to the VM.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := lima.ShowSSH(config.CurrentProfile().ID, sshConfigCmdArgs.layer, sshConfigCmdArgs.format)
		if err == nil {
			fmt.Println(resp.Output)
		}
		return err
	},
}

var sshConfigCmdArgs struct {
	format string
	layer  bool
}

func init() {
	root.Cmd().AddCommand(sshConfigCmd)

	sshConfigCmd.Flags().StringVarP(&sshConfigCmdArgs.format, "format", "f", "config", "format (config, cmd)")
	sshConfigCmd.Flags().BoolVarP(&sshConfigCmdArgs.layer, "layer", "l", true, "config for the Ubuntu layer (if enabled)")
}

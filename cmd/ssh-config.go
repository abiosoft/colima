package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var sshConfigCmd = &cobra.Command{
	Use:   "ssh-config [profile]",
	Short: "show SSH connection config",
	Long:  `Show configuration of the SSH connection to the VM.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := limautil.ShowSSH(config.CurrentProfile().ID)
		if err == nil {
			fmt.Println(resp.Output)
		}
		return err
	},
}

func init() {
	root.Cmd().AddCommand(sshConfigCmd)
}

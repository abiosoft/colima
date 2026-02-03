package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/environment/vm/apple/appleutil"
	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:     "ssh",
	Aliases: []string{"exec", "x"},
	Short:   "SSH into the VM",
	Long: `SSH into the VM.

Appending additional command runs the command instead.
e.g. 'colima ssh -- htop' will run htop.

It is recommended to specify '--' to differentiate from colima flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if this is an Apple Container instance
		if appleutil.IsAppleBackend() {
			return fmt.Errorf("ssh is not supported for Apple Container runtime")
		}
		return newApp().SSH(args...)
	},
}

func init() {
	root.Cmd().AddCommand(sshCmd)
}

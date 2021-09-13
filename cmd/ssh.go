package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH into the VM",
	Long: `SSH into the VM.

Appending with any additional command runs the command instead.
e.g. 'colima ssh htop' will run htop'`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("ssh called")
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)

	// sshCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

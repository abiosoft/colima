package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the VM",
	Long: `Stop stops the VM to free up resources.

The state of the VM is persisted at stop. A start afterwards
should return it back to its previous state.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("stop called")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

}

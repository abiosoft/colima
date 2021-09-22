package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var nerdctlConf struct {
	force bool
}

// nerdctlCmd represents the nerdctl command
var nerdctlCmd = &cobra.Command{
	Use:     "nerdctl",
	Aliases: []string{"nerd", "n"},
	Short:   "Run nerdctl (requires containerd runtime)",
	Long: `Run nerdctl to interact with containerd.
This requires containerd runtime.

It is recommended to specify '--' to differentiate from Colima flags.
`,
	Run: func(cmd *cobra.Command, args []string) {
		nerdctlArgs := append([]string{"sudo", "nerdctl"}, args...)
		cobra.CheckErr(app.SSH(nerdctlArgs...))
	},
}

// nerdctlLink represents the nerdctl command
var nerdctlLink = &cobra.Command{
	Use:   "install",
	Short: "Install nerdctl binary on the host",
	Long: `Install nerdctl binary on the host.
The binary will be installed at /usr/local/bin/nerdctl.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(os.Args)
		fmt.Println("nerdctl install")
	},
}

func init() {
	rootCmd.AddCommand(nerdctlCmd)
	nerdctlCmd.AddCommand(nerdctlLink)
	nerdctlLink.Flags().BoolVarP(&nerdctlConf.force, "force", "f", false, "replace /usr/local/bin/nerdctl (if exists)")
}

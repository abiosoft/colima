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
	Short:   "Run nerdctl (requires --runtime=containerd)",
	Long: `Run nerdctl to interact with containerd.
This requires containerd runtime (--runtime=containerd).

It is recommended to specify '--' to differentiate from colima flags.
`,
	Run: func(cmd *cobra.Command, args []string) {
		nerdctlArgs := append([]string{"sudo", "nerdctl"}, args...)
		cobra.CheckErr(app.SSH(nerdctlArgs...))
	},
}

// nerdctlLink represents the nerdctl command
var nerdctlLink = &cobra.Command{
	Use:   "link",
	Short: "Link nerdctl binary for easy access",
	Long: `Link nerdctl binary for easy access from the host.
This installs the binary in /usr/local/bin/nerdctl.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(os.Args)
		fmt.Println("nerdctl link")
	},
}

func init() {
	rootCmd.AddCommand(nerdctlCmd)
	nerdctlCmd.AddCommand(nerdctlLink)
	nerdctlLink.Flags().BoolVarP(&nerdctlConf.force, "force", "f", false, "replace /usr/local/bin/nerdctl (if exists)")
}

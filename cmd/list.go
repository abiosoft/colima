package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// listCmd represents the version command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list instances",
	Long: `List all created instances.

A new instance can be created during 'colima start' by specifying the '--profile' flag.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		instances, err := lima.Instances()
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
		fmt.Fprintln(w, "PROFILE\tSTATUS\tARCH\tCPUS\tMEMORY\tDISK\tADDRESS")

		if len(instances) == 0 {
			logrus.Warn("No instance found. Run `colima start` to create an instance.")
		}

		for _, inst := range instances {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
				inst.Name,
				inst.Status,
				inst.Arch,
				inst.CPU,
				units.BytesSize(float64(inst.Memory)),
				units.BytesSize(float64(inst.Disk)),
				inst.IPAddress,
			)
		}

		return w.Flush()
	},
}

func init() {
	root.Cmd().AddCommand(listCmd)
}

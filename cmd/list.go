package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listCmdArgs struct {
	json bool
}

// listCmd represents the version command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list instances",
	Long: `List all created instances.

A new instance can be created during 'colima start' by specifying the '--profile' flag.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := []string{}
		if cmd.Flag("profile").Changed {
			profile = append(profile, config.CurrentProfile().ID)
		}

		instances, err := limautil.Instances(profile...)
		if err != nil {
			return err
		}

		if listCmdArgs.json {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			// print instance per line to conform with Lima's output
			for _, instance := range instances {
				// dir should be hidden from the output
				instance.Dir = ""
				if err := encoder.Encode(instance); err != nil {
					return err
				}
			}
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
		_, _ = fmt.Fprintln(w, "PROFILE\tSTATUS\tARCH\tCPUS\tMEMORY\tDISK\tRUNTIME\tADDRESS")

		if len(instances) == 0 {
			logrus.Warn("No instance found. Run `colima start` to create an instance.")
		}

		for _, inst := range instances {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				inst.Name,
				inst.Status,
				inst.Arch,
				inst.CPU,
				units.BytesSize(float64(inst.Memory)),
				units.BytesSize(float64(inst.Disk)),
				inst.Runtime,
				inst.IPAddress,
			)
		}

		return w.Flush()
	},
}

func init() {
	root.Cmd().AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listCmdArgs.json, "json", "j", false, "print json output")
}

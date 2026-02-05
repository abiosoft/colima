package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	// Register VM backends for instance listing
	_ "github.com/abiosoft/colima/environment/vm/apple"
	_ "github.com/abiosoft/colima/environment/vm/lima"
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
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := config.CurrentProfile()
		profileArgs := []string{}

		if profile.Changed {
			profileArgs = append(profileArgs, profile.ID)
		}

		// Get instances from all backends
		instances, err := vm.AllInstances(profileArgs...)
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

		// only show the backend column if an apple runtime instance exists
		var showBackend bool
		for _, inst := range instances {
			if inst.Backend == string(vm.BackendApple) {
				showBackend = true
				break
			}
		}

		columns := []column{
			{header: "PROFILE", value: func(i vm.InstanceInfo) string { return i.Name }},
			{header: "STATUS", value: func(i vm.InstanceInfo) string { return i.Status }},
			{header: "BACKEND", value: func(i vm.InstanceInfo) string { return i.Backend }, hidden: !showBackend},
			{header: "ARCH", value: func(i vm.InstanceInfo) string { return i.Arch }},
			{header: "CPUS", value: func(i vm.InstanceInfo) string {
				if i.CPU < 0 {
					return "N/A"
				}
				return fmt.Sprintf("%d", i.CPU)
			}},
			{header: "MEMORY", value: func(i vm.InstanceInfo) string {
				if i.Memory < 0 {
					return "N/A"
				}
				return units.BytesSize(float64(i.Memory))
			}},
			{header: "DISK", value: func(i vm.InstanceInfo) string {
				if i.Disk < 0 {
					return "N/A"
				}
				return units.BytesSize(float64(i.Disk))
			}},
			{header: "RUNTIME", value: func(i vm.InstanceInfo) string { return i.Runtime }},
			{header: "ADDRESS", value: func(i vm.InstanceInfo) string { return i.IPAddress }},
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 4, 8, 4, ' ', 0)
		printRow(w, columns, func(c column) string { return c.header })

		if len(instances) == 0 {
			logrus.Warn("No instance found. Run `colima start` to create an instance.")
		}

		for _, inst := range instances {
			printRow(w, columns, func(c column) string { return c.value(inst) })
		}

		return w.Flush()
	},
}

type column struct {
	header string
	value  func(vm.InstanceInfo) string
	hidden bool
}

func printRow(w io.Writer, columns []column, value func(column) string) {
	var vals []string
	for _, c := range columns {
		if !c.hidden {
			vals = append(vals, value(c))
		}
	}
	_, _ = fmt.Fprintln(w, strings.Join(vals, "\t"))
}

func init() {
	root.Cmd().AddCommand(listCmd)

	listCmd.Flags().BoolVarP(&listCmdArgs.json, "json", "j", false, "print json output")
}

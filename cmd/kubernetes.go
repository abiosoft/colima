package cmd

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container/kubernetes"

	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"kube", "k8s", "k"},
	Short:   "manage Kubernetes cluster",
	Long:    `Manage the Kubernetes cluster`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// cobra overrides PersistentPreRunE when redeclared.
		// re-run rootCmd's.
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if !newApp().Active() {
			return fmt.Errorf("%s is not running", config.AppName())
		}
		return nil
	},
}

// kubernetesStartCmd represents the kubernetes start command
var kubernetesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start the Kubernetes cluster",
	Long:  `Start the Kubernetes cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}

		if err := k.Provision(); err != nil {
			return err
		}

		return k.Start()
	},
}

// kubernetesStopCmd represents the kubernetes stop command
var kubernetesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the Kubernetes cluster",
	Long:  `Stop the Kubernetes cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}
		if k.Version() == "" {
			return fmt.Errorf("%s is not enabled", kubernetes.Name)
		}

		return k.Stop()
	},
}

// kubernetesDeleteCmd represents the kubernetes delete command
var kubernetesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete the Kubernetes cluster",
	Long:  `Delete the Kubernetes cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}
		if k.Version() == "" {
			return fmt.Errorf("%s is not enabled", kubernetes.Name)
		}

		return k.Teardown()
	},
}

// kubernetesResetCmd represents the kubernetes reset command
var kubernetesResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset the Kubernetes cluster",
	Long: `Reset the Kubernetes cluster.

This resets the Kubernetes cluster and all Kubernetes objects
will be deleted.

The Kubernetes images are cached making the startup (after reset) much faster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}

		if err := k.Teardown(); err != nil {
			return fmt.Errorf("error deleting %s: %w", kubernetes.Name, err)
		}

		if err := k.Provision(); err != nil {
			return err
		}

		if err := k.Start(); err != nil {
			return fmt.Errorf("error starting %s: %w", kubernetes.Name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(kubernetesCmd)
	kubernetesCmd.AddCommand(kubernetesStartCmd)
	kubernetesCmd.AddCommand(kubernetesStopCmd)
	kubernetesCmd.AddCommand(kubernetesDeleteCmd)
	kubernetesCmd.AddCommand(kubernetesResetCmd)
}

package cmd

import (
	"context"
	"fmt"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container/kubernetes"

	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"kube", "k8s", "k3s", "k"},
	Short:   "manage Kubernetes cluster",
	Long:    `Manage the Kubernetes cluster`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// cobra overrides PersistentPreRunE when redeclared.
		// re-run rootCmd's.
		if err := root.Cmd().PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if !newApp().Active() {
			return fmt.Errorf("%s is not running", config.CurrentProfile().DisplayName)
		}
		return nil
	},
}

// kubernetesStartCmd represents the kubernetes start command
var kubernetesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start the Kubernetes cluster",
	Long:  `Start the Kubernetes cluster.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}

		if err := k.Provision(context.Background()); err != nil {
			return err
		}

		return k.Start(context.Background())
	},
}

// kubernetesStopCmd represents the kubernetes stop command
var kubernetesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the Kubernetes cluster",
	Long:  `Stop the Kubernetes cluster.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}
		if !k.Running(ctx) {
			return fmt.Errorf("%s is not enabled", kubernetes.Name)
		}

		return k.Stop(ctx)
	},
}

// kubernetesDeleteCmd represents the kubernetes delete command
var kubernetesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete the Kubernetes cluster",
	Long:  `Delete the Kubernetes cluster.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}
		if !k.Running(ctx) {
			return fmt.Errorf("%s is not enabled", kubernetes.Name)
		}

		return k.Teardown(ctx)
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
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}

		if err := k.Teardown(context.Background()); err != nil {
			return fmt.Errorf("error deleting %s: %w", kubernetes.Name, err)
		}

		ctx := context.Background()
		if err := k.Provision(ctx); err != nil {
			return err
		}

		if err := k.Start(ctx); err != nil {
			return fmt.Errorf("error starting %s: %w", kubernetes.Name, err)
		}

		return nil
	},
}

func init() {
	root.Cmd().AddCommand(kubernetesCmd)
	kubernetesCmd.AddCommand(kubernetesStartCmd)
	kubernetesCmd.AddCommand(kubernetesStopCmd)
	kubernetesCmd.AddCommand(kubernetesDeleteCmd)
	kubernetesCmd.AddCommand(kubernetesResetCmd)
}

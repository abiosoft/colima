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

// kubernetesEnableCmd represents the kubernetes start command
var kubernetesEnableCmd = &cobra.Command{
	Use:     "enable",
	Aliases: []string{"e"},
	Short:   "enable and start the Kubernetes cluster",
	Long:    `Enable and start the Kubernetes cluster.`,
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

// kubernetesDisableCmd represents the kubernetes stop command
var kubernetesDisableCmd = &cobra.Command{
	Use:     "disable",
	Aliases: []string{"d"},
	Short:   "disable and delete the Kubernetes cluster",
	Long:    `Disable and delete the Kubernetes cluster.`,
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

// kubernetesDashboardCmd represents the kubernetes dashboard command
var kubernetesDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"d"},
	Short:   "enable the Kubernetes dashboard and print dashboard url",
	Long: `Enable the Kubernetes dashboard and print dashboard url.

This may take a while on first run, the dashboard is not enabled by default.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		k, err := app.Kubernetes()
		if err != nil {
			return err
		}
		if k.Version() == "" {
			return fmt.Errorf("%s is not enabled", kubernetes.Name)
		}
		return app.SSH("minikube", "dashboard", "--url")
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
		if err := k.Start(); err != nil {
			return fmt.Errorf("error starting %s: %w", kubernetes.Name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(kubernetesCmd)
	kubernetesCmd.AddCommand(kubernetesEnableCmd)
	kubernetesCmd.AddCommand(kubernetesDisableCmd)
	kubernetesCmd.AddCommand(kubernetesDashboardCmd)
	kubernetesCmd.AddCommand(kubernetesResetCmd)
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"kube", "k8s", "k"},
	Short:   "Manage Kubernetes cluster",
	Long:    `Manage the Kubernetes cluster`,
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Kubernetes cluster",
	Long:  `Start the Kubernetes cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Kubernetes cluster",
	Long:  `Stop the Kubernetes cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"d"},
	Short:   "Enable the Kubernetes dashboard",
	Long: `Enable the Kubernetes dashboard.

This may take a while on first run, the dashboard is not enabled by default.

The dashboard url is printed afterwards`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the Kubernetes cluster",
	Long: `Reset the Kubernetes cluster.

This resets the Kubernetes cluster and all Kubernetes objects
will be deleted.

The Kubernetes images are cached making the startup (after reset) much faster.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

func init() {
	rootCmd.AddCommand(kubernetesCmd)
	kubernetesCmd.AddCommand(kubernetesStartCmd)
	kubernetesCmd.AddCommand(kubernetesStopCmd)
	kubernetesCmd.AddCommand(kubernetesDashboardCmd)
	kubernetesCmd.AddCommand(kubernetesResetCmd)
}

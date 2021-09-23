package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// kubernetesCmd represents the kubernetes command
var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"kube", "k8s", "k"},
	Short:   "manage Kubernetes cluster",
	Long:    `Manage the Kubernetes cluster`,
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start the Kubernetes cluster",
	Long:  `Start the Kubernetes cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the Kubernetes cluster",
	Long:  `Stop the Kubernetes cluster.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"d"},
	Short:   "enable the Kubernetes dashboard and print dashboard url",
	Long: `Enable the Kubernetes dashboard and print dashboard url.

This may take a while on first run, the dashboard is not enabled by default.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubernetes called")
	},
}

// kubernetesStartCmd represents the kubernetes command
var kubernetesResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset the Kubernetes cluster",
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

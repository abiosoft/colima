package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/model"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/spf13/cobra"
)

// modelCmdArgs holds command-line flags for the model command.
var modelCmdArgs struct {
	Runner    string
	ServePort int
}

// modelCmd represents the model command
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "manage AI models (requires docker runtime and krunkit VM type)",
	Long: `Manage AI models inside the VM.
This requires docker runtime and krunkit VM type for GPU access.

Use --runner to select the model runner:
  - docker: Docker Model Runner (default)
  - ramalama: Ramalama

All arguments are passed to the selected AI model runner.
Specifying '--' will pass arguments to the underlying tool.

Examples:
  colima model list
  colima model pull ai/smollm2
  colima model run ai/smollm2
  colima model serve
  colima model serve ai/smollm2 --port 8080

Multiple registries are supported.
`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		runner, err := getModelRunner()
		if err != nil {
			return err
		}
		return runner.ValidatePrerequisites(newApp())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		runner, err := getModelRunner()
		if err != nil {
			return err
		}

		a := newApp()

		if err := runner.EnsureProvisioned(); err != nil {
			return err
		}

		runnerArgs, err := runner.BuildArgs(args)
		if err != nil {
			return err
		}
		return a.SSH(runnerArgs...)
	},
}

// modelSetupCmd reinstalls the model runner in the VM.
var modelSetupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "install or update AI model runner in the VM",
	Long:    `Install or update AI model runner and its dependencies in the VM.`,
	Aliases: []string{"update"},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		runner, err := getModelRunner()
		if err != nil {
			return err
		}
		return runner.ValidatePrerequisites(newApp())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		runner, err := getModelRunner()
		if err != nil {
			return err
		}

		// Build header for alternate screen
		var header string
		separator := "────────────────────────────────────────"
		if runner.Name() == model.RunnerDocker {
			header = fmt.Sprintf("Colima - Docker Model Runner Setup\n%s", separator)
		} else {
			header = fmt.Sprintf("Colima - Ramalama Setup\n%s", separator)
		}

		// Run in alternate screen with header
		return terminal.WithAltScreen(func() error {
			return runner.Setup()
		}, header)
	},
}

// modelServeCmd serves a model API.
var modelServeCmd = &cobra.Command{
	Use:   "serve [model]",
	Short: "serve a model API",
	Long: `Serve a model API.

This starts a model server providing:
  - OpenAI-compatible API at http://localhost:<port>/v1
  - Web UI for chat at http://localhost:<port>

Press Ctrl-C to stop the server.
`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		runner, err := getModelRunner()
		if err != nil {
			return err
		}
		return runner.ValidatePrerequisites(newApp())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		runner, err := getModelRunner()
		if err != nil {
			return err
		}

		// Determine the model to serve
		var modelName string
		if len(args) > 0 {
			modelName = args[0]
		} else if runner.Name() == model.RunnerDocker {
			// For docker runner, get the first available model
			firstModel, err := model.GetFirstModel()
			if err != nil {
				return err
			}
			if firstModel == "" {
				return fmt.Errorf("no models available\nPull a model first: colima model pull ai/smollm2")
			}
			modelName = firstModel
		} else {
			return fmt.Errorf("model name is required for ramalama runner\nUsage: colima model serve <model>")
		}

		if err := runner.EnsureProvisioned(); err != nil {
			return err
		}

		// Ensure the model is available (pull if necessary) - this happens outside alternate screen
		normalizedModel, err := runner.EnsureModel(modelName)
		if err != nil {
			return err
		}

		// Determine the port to use
		port := modelCmdArgs.ServePort
		portExplicitlySet := cmd.Flags().Changed("port")

		// If port was not explicitly set, find an available port starting from the default
		const maxPortAttempts = 20
		if !portExplicitlySet {
			availablePort, found := util.FindAvailablePort(port, maxPortAttempts)
			if !found {
				return fmt.Errorf("no available port found in range %d-%d", port, port+maxPortAttempts-1)
			}
			if availablePort != port {
				fmt.Printf("Port %d is in use, using port %d instead\n", port, availablePort)
			}
			port = availablePort
		} else {
			// User explicitly set the port, check if it's available
			if _, found := util.FindAvailablePort(port, 1); !found {
				return fmt.Errorf("port %d is already in use", port)
			}
		}

		// Build header for alternate screen
		separator := "────────────────────────────────────────"
		header := fmt.Sprintf("Colima - Model Server (Ctrl-C to stop)\nWeb UI & API at http://localhost:%d\n%s", port, separator)

		// Run in alternate screen with header
		return terminal.WithAltScreen(func() error {
			return runner.Serve(normalizedModel, port)
		}, header)
	},
}

func init() {
	root.Cmd().AddCommand(modelCmd)
	modelCmd.AddCommand(modelSetupCmd)
	modelCmd.AddCommand(modelServeCmd)

	// Add --runner flag with default from config or ramalama
	modelCmd.PersistentFlags().StringVar(&modelCmdArgs.Runner, "runner", "", "AI model runner (docker, ramalama)")

	// Add --port flag for serve command
	modelServeCmd.Flags().IntVar(&modelCmdArgs.ServePort, "port", 8080, "port for the web UI")
}

// getModelRunner returns the appropriate runner based on flag or config.
func getModelRunner() (model.Runner, error) {
	runnerType := modelCmdArgs.Runner

	// If not specified via flag, check instance config
	if runnerType == "" {
		if conf, err := configmanager.LoadInstance(); err == nil && conf.ModelRunner != "" {
			runnerType = conf.ModelRunner
		}
	}

	// Default to docker
	if runnerType == "" {
		runnerType = string(model.RunnerDocker)
	}

	return model.GetRunner(model.RunnerType(runnerType))
}

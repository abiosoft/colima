package cmd

import (
	"fmt"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/store"
	"github.com/abiosoft/colima/util"
	"github.com/spf13/cobra"
)

// gpuSubcommands are ramalama subcommands that need GPU device passthrough.
var gpuSubcommands = map[string]bool{
	"run":        true,
	"serve":      true,
	"bench":      true,
	"chat":       true,
	"perplexity": true,
}

// modelCmd represents the model command
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "manage AI models (requires docker runtime and krunkit VM type)",
	Long: `Manage AI models inside the VM.
This requires docker runtime and krunkit VM type for GPU access.

All arguments are passed to AI model runner (ramalama).
You can specify '--' to interact directly with the underlying ramalama command.

Examples:
  colima model list
  colima model pull ollama://tinyllama
  colima model run ollama://tinyllama
  colima model serve ollama://tinyllama
  colima model -- --help
`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateModelPrerequisites()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		a := newApp()

		if err := ensureRamalamaProvisioned(); err != nil {
			return err
		}

		ramalamaArgs := buildRamalamaArgs(args)
		return a.SSH(ramalamaArgs...)
	},
}

// modelSetupCmd forces re-provisioning of ramalama in the VM.
var modelSetupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "install or update AI model runner in the VM",
	Long:    `Install or update AI model runner and its dependencies in the VM.`,
	Aliases: []string{"update"},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateModelPrerequisites()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return provisionRamalama()
	},
}

func init() {
	root.Cmd().AddCommand(modelCmd)
	modelCmd.AddCommand(modelSetupCmd)
}

// validateModelPrerequisites checks that the VM is running with the correct
// configuration for AI model support.
func validateModelPrerequisites() error {
	a := newApp()

	// VM must be running
	if !a.Active() {
		return fmt.Errorf("%s is not running", config.CurrentProfile().DisplayName)
	}

	// check runtime is docker
	r, err := a.Runtime()
	if err != nil {
		return err
	}
	if r != docker.Name {
		return fmt.Errorf("'colima model' requires docker runtime, current runtime is %s\n"+
			"Start colima with: colima start --runtime docker --vm-type krunkit", r)
	}

	// check VM type is krunkit
	conf, err := configmanager.LoadInstance()
	if err != nil {
		return fmt.Errorf("error loading instance config: %w", err)
	}
	if conf.VMType != limaconfig.Krunkit {
		return fmt.Errorf("'colima model' requires krunkit VM type for GPU access, current VM type is %s\n"+
			"Start colima with: colima start --runtime docker --vm-type krunkit", conf.VMType)
	}

	// check krunkit binary exists on host
	if err := util.AssertKrunkit(); err != nil {
		return err
	}

	return nil
}

// ensureRamalamaProvisioned checks if ramalama has been provisioned in the VM
// and provisions it if not.
func ensureRamalamaProvisioned() error {
	s, _ := store.Load()
	if s.RamalamaProvisioned {
		return nil
	}

	if !cli.Prompt("AI model support requires initial setup (this may take a few minutes depending on internet connection speed). Continue") {
		return fmt.Errorf("setup cancelled")
	}

	return provisionRamalama()
}

// provisionRamalama installs ramalama and its dependencies in the VM.
func provisionRamalama() error {
	guest := lima.New(host.New())

	script := `set -e
export PATH="$HOME/.local/bin:$PATH"

# install ramalama
curl -fsSL https://ramalama.ai/install.sh | bash

# pull ramalama container images
docker pull quay.io/ramalama/ramalama
docker pull quay.io/ramalama/ramalama-rag

# fix ownership of persistent data dir and symlink to expected location
sudo chown -R $(id -u):$(id -g) /var/lib/ramalama
mkdir -p "$HOME/.local/share"
ln -sfn /var/lib/ramalama "$HOME/.local/share/ramalama"
`

	if err := guest.Run("sh", "-c", script); err != nil {
		return fmt.Errorf("error provisioning ramalama: %w", err)
	}

	// mark as provisioned
	if err := store.Set(func(s *store.Store) {
		s.RamalamaProvisioned = true
	}); err != nil {
		return fmt.Errorf("error saving provisioning state: %w", err)
	}

	return nil
}

// buildRamalamaArgs constructs the full argument list for running ramalama
// in the VM, including environment variables and device passthrough.
// Uses sh -c with "$@" pattern so $HOME and $PATH are properly expanded.
func buildRamalamaArgs(args []string) []string {
	shellCmd := `export RAMALAMA_CONTAINER_ENGINE=docker PATH="$HOME/.local/bin:$PATH"; exec ramalama "$@"`

	ramalamaArgs := []string{"sh", "-c", shellCmd, "--"}

	// for GPU subcommands, inject --device=/dev/dri after the subcommand name
	if len(args) > 0 && gpuSubcommands[args[0]] {
		ramalamaArgs = append(ramalamaArgs, args[0], "--device=/dev/dri")
		ramalamaArgs = append(ramalamaArgs, args[1:]...)
	} else {
		ramalamaArgs = append(ramalamaArgs, args...)
	}

	return ramalamaArgs
}

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	"github.com/abiosoft/colima/util/terminal"
	"github.com/coreos/go-semver/semver"
	log "github.com/sirupsen/logrus"
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

All arguments are passed to the AI model runner.
You can specify '--' to pass arguments directly to the underlying tool.

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
		return setupOrUpdateRamalama()
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

const ramalamaReleasesURL = "https://api.github.com/repos/containers/ramalama/releases/latest"

// setupOrUpdateRamalama handles both fresh installs and updates with version checking.
func setupOrUpdateRamalama() error {
	s, _ := store.Load()

	// Fresh install - no version check needed
	if !s.RamalamaProvisioned {
		if err := provisionRamalama(); err != nil {
			return err
		}
		// Print installed version
		if version := getRamalamaVersion(); version != "" {
			fmt.Println("AI model runner")
			fmt.Printf("version: %s", version)
			fmt.Println()
		}
		return nil
	}

	// Update - check versions first
	currentVersion := getRamalamaVersion()
	if currentVersion == "" {
		// Can't determine current version, proceed with update
		log.Debug("could not determine current ramalama version, proceeding with update")
		return provisionRamalama()
	}

	latestVersion, err := getLatestRamalamaVersion()
	if err != nil {
		log.Debugf("could not fetch latest ramalama version: %v", err)
		return fmt.Errorf("could not check for updates: %w", err)
	}

	// Compare versions
	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		log.Debugf("could not parse current version %q: %v", currentVersion, err)
		return provisionRamalama()
	}

	latest, err := semver.NewVersion(latestVersion)
	if err != nil {
		log.Debugf("could not parse latest version %q: %v", latestVersion, err)
		return provisionRamalama()
	}

	// Show version info
	fmt.Println("AI model runner")
	fmt.Printf("current: %s", currentVersion)
	fmt.Println()
	fmt.Printf("latest:  %s", latestVersion)
	fmt.Println()

	if current.Compare(*latest) >= 0 {
		fmt.Println()
		fmt.Println("Already up to date")
		return nil
	}

	if err := provisionRamalama(); err != nil {
		return err
	}

	// Print new version
	if newVersion := getRamalamaVersion(); newVersion != "" {
		fmt.Printf("updated: %s", newVersion)
		fmt.Println()
	}
	return nil
}

// getRamalamaVersion returns the currently installed ramalama version in the VM.
// Returns empty string if ramalama is not installed or version cannot be determined.
func getRamalamaVersion() string {
	guest := lima.New(host.New())
	output, err := guest.RunOutput("sh", "-c", `export PATH="$HOME/.local/bin:$PATH"; ramalama version 2>/dev/null`)
	if err != nil {
		return ""
	}
	// Output format: "ramalama version 0.17.1"
	output = strings.TrimSpace(output)
	if version, ok := strings.CutPrefix(output, "ramalama version "); ok {
		return version
	}
	return ""
}

// getLatestRamalamaVersion fetches the latest release version from GitHub.
func getLatestRamalamaVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ramalamaReleasesURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Tag might be "v0.17.1" or "0.17.1"
	version := strings.TrimPrefix(release.TagName, "v")
	return version, nil
}

// provisionRamalama installs ramalama and its dependencies in the VM.
func provisionRamalama() error {
	guest := lima.New(host.New())

	log.Println("Installing AI model runner...")

	// step 1: Install ramalama binary (uses normal scrolling output)
	installScript := `set -e
export PATH="$HOME/.local/bin:$PATH"
curl -fsSL https://ramalama.ai/install.sh | bash
`
	if err := guest.Run("sh", "-c", installScript); err != nil {
		return fmt.Errorf("error installing AI model runner: %w", err)
	}

	// step 2: Pull container images (uses alternate screen for progress bars)
	pullScript := `set -e
docker pull quay.io/ramalama/ramalama
docker pull quay.io/ramalama/ramalama-rag
`
	if err := terminal.WithAltScreen(func() error {
		log.Println()
		log.Println("  Colima - AI Model Runner Setup")
		log.Println("  ===============================")
		log.Println()
		log.Println("  Pulling container images...")
		log.Println("  This may take a few minutes depending on your internet connection.")
		log.Println()
		return guest.RunInteractive("sh", "-c", pullScript)
	}); err != nil {
		return fmt.Errorf("error pulling container images: %w", err)
	}

	log.Println("Configuring AI model runner...")

	// step 3: Post-install setup (uses normal scrolling output)
	setupScript := `set -e
sudo chown -R $(id -u):$(id -g) /var/lib/ramalama
mkdir -p "$HOME/.local/share"
ln -sfn /var/lib/ramalama "$HOME/.local/share/ramalama"
`
	if err := guest.Run("sh", "-c", setupScript); err != nil {
		return fmt.Errorf("error configuring AI model runner: %w", err)
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

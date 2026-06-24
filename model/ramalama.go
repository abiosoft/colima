package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/store"
	log "github.com/sirupsen/logrus"
)

const ramalamaReleasesURL = "https://api.github.com/repos/containers/ramalama/releases/latest"

// SetupOrUpdateRamalama installs or updates ramalama.
// Call CheckSetup() first to determine if setup is needed and display version info.
func SetupOrUpdateRamalama() error {
	return ProvisionRamalama()
}

// GetRamalamaVersion returns the currently installed ramalama version in the VM.
// Returns empty string if ramalama is not installed or version cannot be determined.
func GetRamalamaVersion() string {
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

// ramalamaModel represents a model from ramalama ls --json output.
type ramalamaModel struct {
	Name     string `json:"name"`
	Modified string `json:"modified"`
	Size     int64  `json:"size"`
}

// listRamalamaModels returns all locally available ramalama models.
func listRamalamaModels() ([]ramalamaModel, error) {
	guest := lima.New(host.New())
	output, err := guest.RunOutput("sh", "-c", `export PATH="$HOME/.local/bin:$PATH"; ramalama ls --json 2>/dev/null`)
	if err != nil {
		return nil, fmt.Errorf("error listing models: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return nil, nil
	}

	var models []ramalamaModel
	if err := json.Unmarshal([]byte(output), &models); err != nil {
		return nil, fmt.Errorf("error parsing model list: %w", err)
	}

	return models, nil
}

// ramalamaModelExists checks if a model exists locally in ramalama.
func ramalamaModelExists(modelName string) bool {
	models, err := listRamalamaModels()
	if err != nil {
		return false
	}

	// Normalize the input model name
	normalizedInput := normalizeRamalamaModelName(modelName)

	for _, m := range models {
		// Model names in ramalama have format like "hf://TheBloke/..." or "ollama://library/..."
		normalizedStored := normalizeRamalamaModelName(m.Name)
		if normalizedInput == normalizedStored {
			return true
		}
	}

	return false
}

// normalizeRamalamaModelName normalizes a ramalama model name for comparison.
func normalizeRamalamaModelName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Normalize different URL formats to a common form
	// "hf.co/..." -> "hf://..."
	// "huggingface.co/..." -> "hf://..."
	name = strings.ReplaceAll(name, "hf.co/", "hf://")
	name = strings.ReplaceAll(name, "huggingface.co/", "hf://")

	return name
}

// EnsureRamalamaModel ensures a ramalama model is available, pulling if necessary.
func EnsureRamalamaModel(modelName string) error {
	if ramalamaModelExists(modelName) {
		return nil
	}

	// Model not found locally, pull it
	guest := lima.New(host.New())
	shellCmd := fmt.Sprintf(
		`export RAMALAMA_CONTAINER_ENGINE=docker PATH="$HOME/.local/bin:$PATH"; ramalama pull %s`,
		modelName,
	)

	if err := guest.RunInteractive("sh", "-c", shellCmd); err != nil {
		return fmt.Errorf("failed to pull model %q: %w", modelName, err)
	}

	return nil
}

// ProvisionRamalama installs ramalama and its dependencies in the VM.
func ProvisionRamalama() error {
	guest := lima.New(host.New())

	script := `set -e
export PATH="$HOME/.local/bin:$PATH"

# ensure pipx is available
sudo apt-get update -y && sudo apt-get install -y pipx

# install ramalama via pipx; upgrade if ramalama is already installed
if command -v ramalama >/dev/null 2>&1; then
  pipx upgrade ramalama
else
  pipx install ramalama
fi

# pull ramalama container images
docker pull quay.io/ramalama/ramalama
docker pull quay.io/ramalama/ramalama-rag

# fix ownership of persistent data dir and symlink to expected location
sudo chown -R $(id -u):$(id -g) /var/lib/ramalama
mkdir -p "$HOME/.local/share"
ln -sfn /var/lib/ramalama "$HOME/.local/share/ramalama"
`

	log.Println("installing AI model runner...")

	if err := guest.RunInteractive("sh", "-c", script); err != nil {
		return fmt.Errorf("error setting up AI model runner: %w", err)
	}

	log.Println("AI model runner installed")

	// mark as provisioned
	if err := store.Set(func(s *store.Store) {
		s.RamalamaProvisioned = true
	}); err != nil {
		return fmt.Errorf("error saving provisioning state: %w", err)
	}

	return nil
}

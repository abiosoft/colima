package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/cli"
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
)

// RunnerType represents the type of AI model runner.
type RunnerType string

const (
	RunnerDocker   RunnerType = "docker"
	RunnerRamalama RunnerType = "ramalama"
)

// SetupStatus contains the result of checking if setup is needed.
type SetupStatus struct {
	// NeedsSetup indicates whether setup/update is required.
	NeedsSetup bool
	// CurrentVersion is the currently installed version (empty if not installed).
	CurrentVersion string
	// LatestVersion is the latest available version (empty if not checked).
	LatestVersion string
}

// Runner defines the interface for AI model runners.
type Runner interface {
	// Name returns the runner type name.
	Name() RunnerType
	// DisplayName returns a human-readable name for the runner.
	DisplayName() string
	// ValidatePrerequisites checks runner-specific requirements.
	ValidatePrerequisites(a app.App) error
	// EnsureProvisioned ensures the runner is set up (no-op for docker).
	EnsureProvisioned() error
	// BuildArgs constructs the command arguments for the runner.
	// Returns an error if the command is not supported.
	BuildArgs(args []string) ([]string, error)
	// EnsureModel ensures a model is available (pulls if necessary).
	// Returns the normalized model name.
	EnsureModel(model string) (string, error)
	// Serve starts serving a model on the given port.
	// This is a blocking call that runs until interrupted.
	// The model should already be available (call EnsureModel first).
	Serve(model string, port int) error
	// CheckSetup checks if setup/update is needed and returns version info.
	// This should be called before Setup() to display version info on primary screen.
	CheckSetup() (SetupStatus, error)
	// Setup installs or updates the runner.
	// Call CheckSetup() first to determine if setup is needed.
	Setup() error
	// GetCurrentVersion returns the currently installed version.
	GetCurrentVersion() string
}

// GetRunner returns the appropriate Runner based on type.
func GetRunner(runnerType RunnerType) (Runner, error) {
	switch runnerType {
	case RunnerDocker:
		return &dockerRunner{}, nil
	case RunnerRamalama:
		return &ramalamaRunner{}, nil
	default:
		return nil, fmt.Errorf("unknown runner type: %s (valid options: docker, ramalama)", runnerType)
	}
}

// validateCommonPrerequisites checks prerequisites common to all runners.
func validateCommonPrerequisites(a app.App) error {
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

	// check VM type is krunkit (required for GPU access)
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

// dockerRunner implements Runner for Docker Model Runner.
type dockerRunner struct{}

func (d *dockerRunner) Name() RunnerType {
	return RunnerDocker
}

func (d *dockerRunner) DisplayName() string {
	return "Docker Model Runner"
}

func (d *dockerRunner) ValidatePrerequisites(a app.App) error {
	return validateCommonPrerequisites(a)
}

func (d *dockerRunner) EnsureProvisioned() error {
	// Docker Model Runner requires no provisioning
	return nil
}

func (d *dockerRunner) BuildArgs(args []string) ([]string, error) {
	// docker model <subcommand> [args...]
	return append([]string{"docker", "model"}, args...), nil
}

// EnsureModel ensures a Docker model is available, pulling if necessary.
// Returns the normalized model name (resolving aliases like hf.co → huggingface.co).
func (d *dockerRunner) EnsureModel(modelName string) (string, error) {
	return EnsureDockerModel(modelName)
}

// Serve starts serving a Docker model using llama-server.
func (d *dockerRunner) Serve(modelName string, port int) error {
	return ServeDockerModel(DockerModelServeConfig{
		ModelName: modelName,
		Port:      port,
	})
}

// dockerModel represents a model from docker model list --json output.
type dockerModel struct {
	ID   string   `json:"id"`
	Tags []string `json:"tags"`
}

// GetFirstModel returns the first available model from docker model list.
// Returns empty string if no models are available.
func GetFirstModel() (string, error) {
	models, err := listDockerModels()
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", nil
	}
	// Return the first tag of the first model
	if len(models[0].Tags) > 0 {
		return models[0].Tags[0], nil
	}
	return "", nil
}

// listDockerModels returns all available models from docker model list.
func listDockerModels() ([]dockerModel, error) {
	guest := lima.New(host.New())
	output, err := guest.RunOutput("docker", "model", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("error listing models: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return nil, nil
	}

	var models []dockerModel
	if err := json.Unmarshal([]byte(output), &models); err != nil {
		return nil, fmt.Errorf("error parsing model list: %w", err)
	}

	return models, nil
}

// ResolveModelName resolves a short model name to its full tag.
// Supports flexible matching:
//   - "smollm2" resolves to "docker.io/ai/smollm2:latest"
//   - "ai/smollm2" resolves to "docker.io/ai/smollm2:latest"
//   - "hf.co/..." resolves to "huggingface.co/..."
//
// Returns the original name if no match is found (for new models to be pulled).
func ResolveModelName(name string) (string, error) {
	models, err := listDockerModels()
	if err != nil {
		return name, err
	}

	for _, m := range models {
		for _, tag := range m.Tags {
			if matchesModel(name, tag) {
				return tag, nil
			}
		}
	}
	// Return original name if not found (will be pulled)
	return name, nil
}

// matchesModel checks if a user-provided name matches a full model tag.
func matchesModel(name, tag string) bool {
	// Normalize both for comparison
	normName := normalizeModelName(name)
	normTag := normalizeModelName(tag)

	// Exact match after normalization
	if normName == normTag {
		return true
	}

	// Check if name is a suffix of tag (e.g., "smollm2" matches "ai/smollm2")
	// Strip the tag version suffix for matching
	tagParts := strings.Split(normTag, ":")
	tagWithoutVersion := tagParts[0]
	tagVersion := ""
	if len(tagParts) > 1 {
		tagVersion = tagParts[1]
	}

	nameParts := strings.Split(normName, ":")
	nameWithoutVersion := nameParts[0]
	nameHasVersion := len(nameParts) > 1

	// If input has no version, only match :latest tags
	if !nameHasVersion && tagVersion != "" && tagVersion != "latest" {
		return false
	}

	// "smollm2" should match "ai/smollm2:latest"
	if strings.HasSuffix(tagWithoutVersion, "/"+normName) {
		return true
	}

	// "ai/smollm2" should match "docker.io/ai/smollm2" or just "ai/smollm2"
	if strings.HasSuffix(tagWithoutVersion, "/"+nameWithoutVersion) {
		return true
	}

	// Direct suffix match (handles cases like "tinyllama/tinyllama-1.1b-chat-v1.0")
	if strings.HasSuffix(tagWithoutVersion, nameWithoutVersion) {
		return true
	}

	return false
}

// normalizeModelName normalizes a model name for comparison.
func normalizeModelName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Normalize registry prefixes
	name = strings.TrimPrefix(name, "docker.io/")
	name = strings.ReplaceAll(name, "hf.co/", "huggingface.co/")

	return name
}

func (d *dockerRunner) CheckSetup() (SetupStatus, error) {
	// Docker Model Runner always reinstalls; no version comparison
	return SetupStatus{
		NeedsSetup:     true,
		CurrentVersion: GetDockerModelVersion(),
	}, nil
}

func (d *dockerRunner) Setup() error {
	return SetupOrUpdateDocker()
}

func (d *dockerRunner) GetCurrentVersion() string {
	return GetDockerModelVersion()
}

// gpuSubcommands are ramalama subcommands that need GPU device passthrough.
var gpuSubcommands = map[string]bool{
	"run":        true,
	"serve":      true,
	"bench":      true,
	"chat":       true,
	"perplexity": true,
}

// ramalamaRunner implements Runner for Ramalama.
type ramalamaRunner struct{}

func (r *ramalamaRunner) Name() RunnerType {
	return RunnerRamalama
}

func (r *ramalamaRunner) DisplayName() string {
	return "Ramalama"
}

func (r *ramalamaRunner) ValidatePrerequisites(a app.App) error {
	return validateCommonPrerequisites(a)
}

func (r *ramalamaRunner) EnsureProvisioned() error {
	s, _ := store.Load()
	if s.RamalamaProvisioned {
		return nil
	}

	prompt := fmt.Sprintf("%s requires initial setup (this may take a few minutes depending on internet connection speed). Continue", r.DisplayName())
	if !cli.Prompt(prompt) {
		return fmt.Errorf("setup cancelled")
	}

	separator := "────────────────────────────────────────"
	header := fmt.Sprintf("Colima - %s Setup\n%s", r.DisplayName(), separator)

	return terminal.WithAltScreen(ProvisionRamalama, header)
}

func (r *ramalamaRunner) BuildArgs(args []string) ([]string, error) {
	return r.buildRamalamaArgs(args), nil
}

// EnsureModel ensures a ramalama model is available, pulling if necessary.
func (r *ramalamaRunner) EnsureModel(modelName string) (string, error) {
	if err := EnsureRamalamaModel(modelName); err != nil {
		return "", err
	}
	return modelName, nil
}

// Serve starts serving a model using ramalama.
func (r *ramalamaRunner) Serve(modelName string, port int) error {
	guest := lima.New(host.New())

	// ramalama serve <model> with GPU support and custom port
	shellCmd := fmt.Sprintf(
		`export RAMALAMA_CONTAINER_ENGINE=docker PATH="$HOME/.local/bin:$PATH"; exec ramalama serve --device=/dev/dri -p %d %s`,
		port, modelName,
	)

	return guest.RunInteractive("sh", "-c", shellCmd)
}

func (r *ramalamaRunner) buildRamalamaArgs(args []string) []string {
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

func (r *ramalamaRunner) CheckSetup() (SetupStatus, error) {
	s, _ := store.Load()

	// Fresh install - no version check needed
	if !s.RamalamaProvisioned {
		return SetupStatus{NeedsSetup: true}, nil
	}

	// Get current version
	currentVersion := GetRamalamaVersion()
	if currentVersion == "" {
		// Can't determine current version, proceed with update
		log.Debug("could not determine current ramalama version, proceeding with update")
		return SetupStatus{NeedsSetup: true}, nil
	}

	// Fetch latest version
	latestVersion, err := getLatestRamalamaVersion()
	if err != nil {
		log.Debugf("could not fetch latest ramalama version: %v", err)
		return SetupStatus{}, fmt.Errorf("could not check for updates: %w", err)
	}

	// Compare versions
	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		log.Debugf("could not parse current version %q: %v", currentVersion, err)
		return SetupStatus{
			NeedsSetup:     true,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
		}, nil
	}

	latest, err := semver.NewVersion(latestVersion)
	if err != nil {
		log.Debugf("could not parse latest version %q: %v", latestVersion, err)
		return SetupStatus{
			NeedsSetup:     true,
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
		}, nil
	}

	needsSetup := current.Compare(*latest) < 0

	return SetupStatus{
		NeedsSetup:     needsSetup,
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
	}, nil
}

func (r *ramalamaRunner) Setup() error {
	return SetupOrUpdateRamalama()
}

func (r *ramalamaRunner) GetCurrentVersion() string {
	return GetRamalamaVersion()
}

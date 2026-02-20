package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/util/terminal"
	log "github.com/sirupsen/logrus"
)

// DockerModelInfo represents the output of docker model inspect.
type DockerModelInfo struct {
	ID     string   `json:"id"`
	Tags   []string `json:"tags"`
	Config struct {
		Format       string `json:"format"`
		Quantization string `json:"quantization"`
		Parameters   string `json:"parameters"`
		Architecture string `json:"architecture"`
		Size         string `json:"size"`
	} `json:"config"`
}

// Hash returns the model's hash (without the "sha256:" prefix).
func (m *DockerModelInfo) Hash() string {
	if hash, ok := strings.CutPrefix(m.ID, "sha256:"); ok {
		return hash
	}
	return ""
}

// ociManifest represents the OCI manifest structure for Docker models.
type ociManifest struct {
	Layers []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

// findGGUFPath finds the GGUF file path for a model inside the docker-model-runner container.
// It handles both Docker registry models (bundle path) and HuggingFace models (blob path via manifest).
// For models without a bundle, it creates the bundle structure by hard-linking the blob.
func findGGUFPath(guest environment.VM, modelHash string) (string, error) {
	// Standard bundle path used by Docker Model Runner for all models
	bundlePath := fmt.Sprintf("/models/bundles/sha256/%s/model/model.gguf", modelHash)

	// Check if bundle already exists
	if err := guest.RunQuiet("docker", "exec", "docker-model-runner", "test", "-f", bundlePath); err == nil {
		return bundlePath, nil
	}

	// Bundle doesn't exist - read manifest to find the GGUF blob and create the bundle
	manifestPath := fmt.Sprintf("/models/manifests/sha256/%s", modelHash)
	output, err := guest.RunOutput("docker", "exec", "docker-model-runner", "cat", manifestPath)
	if err != nil {
		return "", fmt.Errorf("failed to read model manifest: %w", err)
	}

	var manifest ociManifest
	if err := json.Unmarshal([]byte(output), &manifest); err != nil {
		return "", fmt.Errorf("failed to parse model manifest: %w", err)
	}

	// Find the GGUF layer (mediaType contains "gguf")
	var blobPath string
	for _, layer := range manifest.Layers {
		if strings.Contains(layer.MediaType, "gguf") {
			if blobHash, ok := strings.CutPrefix(layer.Digest, "sha256:"); ok {
				blobPath = fmt.Sprintf("/models/blobs/sha256/%s", blobHash)
				break
			}
		}
	}

	if blobPath == "" {
		return "", fmt.Errorf("no GGUF layer found in model manifest")
	}

	// Create bundle directory and hard-link the blob (same approach as Docker Model Runner)
	bundleDir := fmt.Sprintf("/models/bundles/sha256/%s/model", modelHash)
	if err := guest.RunQuiet("docker", "exec", "docker-model-runner", "mkdir", "-p", bundleDir); err != nil {
		return "", fmt.Errorf("failed to create bundle directory: %w", err)
	}

	if err := guest.RunQuiet("docker", "exec", "docker-model-runner", "ln", blobPath, bundlePath); err != nil {
		return "", fmt.Errorf("failed to link model file: %w", err)
	}

	return bundlePath, nil
}

// InspectDockerModel returns information about a Docker model.
func InspectDockerModel(modelName string) (*DockerModelInfo, error) {
	guest := lima.New(host.New())
	output, err := guest.RunOutput("docker", "model", "inspect", modelName)
	if err != nil {
		return nil, fmt.Errorf("error inspecting model %q: %w", modelName, err)
	}

	var info DockerModelInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &info); err != nil {
		return nil, fmt.Errorf("error parsing model info: %w", err)
	}

	return &info, nil
}

// SetupOrUpdateDocker reinstalls Docker Model Runner in the VM.
func SetupOrUpdateDocker() error {
	guest := lima.New(host.New())

	log.Println("reinstalling Docker Model Runner...")

	if err := guest.RunInteractive("docker", "model", "reinstall-runner"); err != nil {
		return fmt.Errorf("error reinstalling Docker Model Runner: %w", err)
	}

	log.Println("Docker Model Runner reinstalled")

	// Print installed version
	if version := GetDockerModelVersion(); version != "" {
		fmt.Println("Docker Model Runner")
		fmt.Printf("version: %s", version)
		fmt.Println()
	}

	return nil
}

// GetDockerModelVersion returns the Docker Model Runner version in the VM.
// Returns empty string if version cannot be determined.
func GetDockerModelVersion() string {
	guest := lima.New(host.New())
	output, err := guest.RunOutput("docker", "model", "version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

// EnsureDockerModel ensures a Docker model is available, pulling if necessary.
// Returns the normalized model name (resolving aliases like hf.co → huggingface.co).
func EnsureDockerModel(modelName string) (string, error) {
	guest := lima.New(host.New())

	// Try to inspect the model first
	modelInfo, err := InspectDockerModel(modelName)
	if err != nil {
		// Model not found locally, try to pull it
		if pullErr := guest.RunInteractive("docker", "model", "pull", modelName); pullErr != nil {
			return "", fmt.Errorf("failed to pull model %q: %w", modelName, pullErr)
		}
		// Retry inspect after pull
		modelInfo, err = InspectDockerModel(modelName)
		if err != nil {
			return "", fmt.Errorf("failed to inspect model %q after pull: %w", modelName, err)
		}
	}

	// Return the first tag as the normalized name (e.g., "docker.io/ai/smollm2:latest")
	if len(modelInfo.Tags) > 0 {
		return modelInfo.Tags[0], nil
	}
	return modelName, nil
}

// DockerModelServeConfig holds configuration for serving a Docker model.
type DockerModelServeConfig struct {
	ModelName string // Model name (e.g., "smollm2")
	Port      int    // Host port to expose the model on
	Threads   int    // Number of CPU threads (default: 2)
	GPULayers int    // Number of GPU layers (default: 999 = all)
}

// ServeDockerModel serves a Docker model with llama-server.
// It runs llama-server interactively (with visible output) and uses socat to forward the port.
// The function blocks until interrupted (Ctrl-C) or llama-server exits.
// Note: Call EnsureDockerModel first to ensure the model is available.
func ServeDockerModel(cfg DockerModelServeConfig) error {
	guest := lima.New(host.New())

	// Set defaults
	if cfg.Threads <= 0 {
		cfg.Threads = 2
	}
	if cfg.GPULayers <= 0 {
		cfg.GPULayers = 999
	}

	// Get the model info (model should already be available via EnsureDockerModel)
	modelInfo, err := InspectDockerModel(cfg.ModelName)
	if err != nil {
		return fmt.Errorf("failed to inspect model %q: %w", cfg.ModelName, err)
	}

	// Check model format - only GGUF models are supported
	if modelInfo.Config.Format != "gguf" {
		return fmt.Errorf("model %q has format %q, only GGUF models are supported\n"+
			"Try a GGUF version of this model (e.g., from TheBloke on HuggingFace)",
			cfg.ModelName, modelInfo.Config.Format)
	}

	modelHash := modelInfo.Hash()
	if modelHash == "" {
		return fmt.Errorf("could not determine hash for model %q", cfg.ModelName)
	}

	// Ensure docker-model-runner container is running (needed to find GGUF path)
	if err := ensureDockerModelRunner(guest); err != nil {
		return err
	}

	// Find the GGUF file path (handles both Docker registry and HuggingFace models)
	ggufPath, err := findGGUFPath(guest, modelHash)
	if err != nil {
		return fmt.Errorf("could not find GGUF file for model %q: %w", cfg.ModelName, err)
	}

	// Get container IP
	containerIP, err := getDockerModelRunnerIP(guest)
	if err != nil {
		return err
	}

	// Kill any existing socat on this port
	stopSocat(guest, cfg.Port)

	// Start socat in background to forward localhost:port → container_ip:port
	if err := startSocat(guest, cfg.Port, containerIP); err != nil {
		return fmt.Errorf("failed to start port forwarder: %w", err)
	}

	// Run llama-server interactively (blocking, with visible output)
	// Ctrl-C will be received by the interactive process directly
	// Use -it for TTY, -i for non-TTY (e.g., piped or CI environments)
	execFlag := "-i"
	if terminal.IsTerminal() {
		execFlag = "-it"
	}

	err = guest.RunInteractive("docker", "exec", execFlag, "docker-model-runner",
		"/app/bin/com.docker.llama-server",
		"-ngl", fmt.Sprintf("%d", cfg.GPULayers),
		"--metrics",
		"--threads", fmt.Sprintf("%d", cfg.Threads),
		"--model", ggufPath,
		"--alias", cfg.ModelName,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", cfg.Port),
		"--jinja",
	)

	// Cleanup socat on exit (whether normal exit or Ctrl-C)
	stopSocat(guest, cfg.Port)

	return err
}

// ensureDockerModelRunner ensures the docker-model-runner container is running.
// Attempts to start it up to 3 times if not found.
func ensureDockerModelRunner(guest environment.VM) error {
	for attempt := 1; attempt <= 3; attempt++ {
		// Check if container exists
		if err := guest.RunQuiet("docker", "inspect", "docker-model-runner"); err == nil {
			return nil
		}

		log.Infof("docker-model-runner not found, starting it (attempt %d/3)...", attempt)
		_ = guest.Run("docker", "model", "start-runner")
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("could not start docker-model-runner after 3 attempts")
}

// getDockerModelRunnerIP returns the IP address of the docker-model-runner container.
func getDockerModelRunnerIP(guest environment.VM) (string, error) {
	output, err := guest.RunOutput("docker", "inspect", "docker-model-runner",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}")
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	ip := strings.TrimSpace(output)
	if ip == "" {
		return "", fmt.Errorf("container IP is empty")
	}

	return ip, nil
}

// startSocat starts socat in the background to forward a port to the container.
func startSocat(guest environment.VM, port int, containerIP string) error {
	cmd := fmt.Sprintf("nohup socat TCP-LISTEN:%d,fork,reuseaddr TCP:%s:%d > /dev/null 2>&1 &",
		port, containerIP, port)
	return guest.Run("sh", "-c", cmd)
}

// stopSocat stops the socat process for a given port.
func stopSocat(guest environment.VM, port int) {
	cmd := fmt.Sprintf("pkill -f 'socat.*TCP-LISTEN:%d' 2>/dev/null || true", port)
	_ = guest.Run("sh", "-c", cmd)
}

// StopDockerModelServe stops a Docker model serve instance.
func StopDockerModelServe(port int) error {
	guest := lima.New(host.New())

	// Stop the socat proxy on the VM
	stopCmd := fmt.Sprintf("pkill -f 'socat.*TCP-LISTEN:%d' 2>/dev/null || true", port)
	if err := guest.Run("sh", "-c", stopCmd); err != nil {
		log.Debugf("error stopping socat: %v", err)
	}

	// Note: llama-server processes inside docker-model-runner are harder to clean up
	// since they run in the same container. For now, we just stop the socat proxy.
	// The llama-server process will remain running but be inaccessible.

	return nil
}

// IsDockerModelServeRunning checks if a serve instance is running on the given port.
func IsDockerModelServeRunning(port int) bool {
	guest := lima.New(host.New())

	// Check if socat is running for this port
	checkCmd := fmt.Sprintf("pgrep -f 'socat.*TCP-LISTEN:%d' > /dev/null 2>&1", port)
	err := guest.Run("sh", "-c", checkCmd)
	return err == nil
}

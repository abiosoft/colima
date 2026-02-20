package model

import (
	"testing"
)

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase conversion",
			input:    "AI/SmollM2",
			expected: "ai/smollm2",
		},
		{
			name:     "trim whitespace",
			input:    "  ai/smollm2  ",
			expected: "ai/smollm2",
		},
		{
			name:     "strip docker.io prefix",
			input:    "docker.io/ai/smollm2",
			expected: "ai/smollm2",
		},
		{
			name:     "convert hf.co to huggingface.co",
			input:    "hf.co/tinyllama/model",
			expected: "huggingface.co/tinyllama/model",
		},
		{
			name:     "already normalized",
			input:    "ai/smollm2:latest",
			expected: "ai/smollm2:latest",
		},
		{
			name:     "huggingface.co unchanged",
			input:    "huggingface.co/tinyllama/model:latest",
			expected: "huggingface.co/tinyllama/model:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeModelName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeModelName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMatchesModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tag      string
		expected bool
	}{
		// Exact matches
		{
			name:     "exact match with full tag",
			input:    "docker.io/ai/smollm2:latest",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},
		{
			name:     "exact match after normalization",
			input:    "ai/smollm2:latest",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},

		// Short name matches
		{
			name:     "short name matches full tag",
			input:    "smollm2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},
		{
			name:     "short name with ai prefix",
			input:    "ai/smollm2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},
		{
			name:     "short name case insensitive",
			input:    "SmollM2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},

		// Huggingface models
		{
			name:     "hf.co prefix matches huggingface.co",
			input:    "hf.co/tinyllama/tinyllama-1.1b-chat-v1.0",
			tag:      "huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
			expected: true,
		},
		{
			name:     "huggingface short name",
			input:    "tinyllama/tinyllama-1.1b-chat-v1.0",
			tag:      "huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
			expected: true,
		},
		{
			name:     "huggingface model name only",
			input:    "tinyllama-1.1b-chat-v1.0",
			tag:      "huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
			expected: true,
		},

		// Version tag handling
		{
			name:     "input without version matches tag with version",
			input:    "ai/smollm2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},
		{
			name:     "input with version matches tag with same version",
			input:    "ai/smollm2:latest",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},
		{
			name:     "input with specific version matches tag with same version",
			input:    "ai/smollm2:v1.0",
			tag:      "docker.io/ai/smollm2:v1.0",
			expected: true,
		},
		{
			name:     "input without version does NOT match tag with specific version",
			input:    "smollm2",
			tag:      "docker.io/ai/smollm2:v1.0",
			expected: false,
		},
		{
			name:     "short name does NOT match tag with specific version",
			input:    "gemma3",
			tag:      "docker.io/ai/gemma3:4b-it-qat-q4_0",
			expected: false,
		},
		{
			name:     "input without version matches tag with latest",
			input:    "smollm2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: true,
		},

		// Non-matches
		{
			name:     "different model names",
			input:    "smollm2",
			tag:      "docker.io/ai/gemma3:latest",
			expected: false,
		},
		{
			name:     "partial name should not match",
			input:    "smoll",
			tag:      "docker.io/ai/smollm2:latest",
			expected: false,
		},
		{
			name:     "different registry",
			input:    "ollama/smollm2",
			tag:      "docker.io/ai/smollm2:latest",
			expected: false,
		},

		// Edge cases
		{
			name:     "gemma3 short name",
			input:    "gemma3",
			tag:      "docker.io/ai/gemma3:latest",
			expected: true,
		},
		{
			name:     "ai/gemma3",
			input:    "ai/gemma3",
			tag:      "docker.io/ai/gemma3:latest",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesModel(tt.input, tt.tag)
			if result != tt.expected {
				t.Errorf("matchesModel(%q, %q) = %v, want %v", tt.input, tt.tag, result, tt.expected)
			}
		})
	}
}

func TestResolveModelNameWithMockData(t *testing.T) {
	// Test the resolution logic by testing matchesModel with various inputs
	// against a set of mock tags that would come from docker model list

	mockTags := []string{
		"docker.io/ai/smollm2:latest",
		"huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
		"docker.io/ai/gemma3:latest",
	}

	tests := []struct {
		name        string
		input       string
		shouldMatch string // empty if no match expected
	}{
		{
			name:        "smollm2 resolves to full tag",
			input:       "smollm2",
			shouldMatch: "docker.io/ai/smollm2:latest",
		},
		{
			name:        "ai/smollm2 resolves to full tag",
			input:       "ai/smollm2",
			shouldMatch: "docker.io/ai/smollm2:latest",
		},
		{
			name:        "gemma3 resolves to full tag",
			input:       "gemma3",
			shouldMatch: "docker.io/ai/gemma3:latest",
		},
		{
			name:        "tinyllama model resolves",
			input:       "tinyllama/tinyllama-1.1b-chat-v1.0",
			shouldMatch: "huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
		},
		{
			name:        "hf.co prefix resolves",
			input:       "hf.co/tinyllama/tinyllama-1.1b-chat-v1.0",
			shouldMatch: "huggingface.co/tinyllama/tinyllama-1.1b-chat-v1.0:latest",
		},
		{
			name:        "unknown model returns no match",
			input:       "unknown-model",
			shouldMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matched string
			for _, tag := range mockTags {
				if matchesModel(tt.input, tag) {
					matched = tag
					break
				}
			}

			if matched != tt.shouldMatch {
				t.Errorf("resolving %q: got %q, want %q", tt.input, matched, tt.shouldMatch)
			}
		})
	}
}

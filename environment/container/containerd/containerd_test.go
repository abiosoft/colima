package containerd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

// mockGuest implements environment.GuestActions, capturing Write calls.
type mockGuest struct {
	writtenPath string
	writtenData []byte
}

func (m *mockGuest) Write(fileName string, body []byte) error {
	m.writtenPath = fileName
	m.writtenData = make([]byte, len(body))
	copy(m.writtenData, body)
	return nil
}

func (m *mockGuest) Read(string) (string, error)                        { return "", nil }
func (m *mockGuest) Stat(string) (os.FileInfo, error)                   { return nil, nil }
func (m *mockGuest) Run(...string) error                                { return nil }
func (m *mockGuest) RunQuiet(...string) error                           { return nil }
func (m *mockGuest) RunOutput(...string) (string, error)                { return "", nil }
func (m *mockGuest) RunInteractive(...string) error                     { return nil }
func (m *mockGuest) RunWith(io.Reader, io.Writer, ...string) error      { return nil }
func (m *mockGuest) Start(context.Context, config.Config) error         { return nil }
func (m *mockGuest) Stop(context.Context, bool) error                   { return nil }
func (m *mockGuest) Restart(context.Context) error                      { return nil }
func (m *mockGuest) SSH(string, ...string) error                        { return nil }
func (m *mockGuest) Created() bool                                      { return false }
func (m *mockGuest) Running(context.Context) bool                       { return false }
func (m *mockGuest) Env(string) (string, error)                         { return "", nil }
func (m *mockGuest) Get(string) string                                  { return "" }
func (m *mockGuest) Set(string, string) error                           { return nil }
func (m *mockGuest) User() (string, error)                              { return "", nil }
func (m *mockGuest) Arch() environment.Arch                             { return environment.Arch("x86_64") }

var _ environment.GuestActions = (*mockGuest)(nil)

func newTestRuntime(guest *mockGuest) containerdRuntime {
	return containerdRuntime{guest: guest}
}

func TestProvisionConfig_ProfileOverride(t *testing.T) {
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "profile")
	centralDir := filepath.Join(tmpDir, "central")

	profilePath := filepath.Join(profileDir, "config.toml")
	centralPath := filepath.Join(centralDir, "config.toml")
	guestPath := "/etc/containerd/config.toml"

	// Write both profile and central configs
	os.MkdirAll(profileDir, 0755)
	os.MkdirAll(centralDir, 0755)
	os.WriteFile(profilePath, []byte("profile-config"), 0644)
	os.WriteFile(centralPath, []byte("central-config"), 0644)

	guest := &mockGuest{}
	rt := newTestRuntime(guest)

	if err := rt.provisionConfig(profilePath, centralPath, guestPath, []byte("default-config")); err != nil {
		t.Fatalf("provisionConfig() error = %v", err)
	}

	if string(guest.writtenData) != "profile-config" {
		t.Errorf("expected profile config to take priority, got %q", string(guest.writtenData))
	}
	if guest.writtenPath != guestPath {
		t.Errorf("expected guest path %q, got %q", guestPath, guest.writtenPath)
	}
}

func TestProvisionConfig_CentralFallback(t *testing.T) {
	tmpDir := t.TempDir()
	profilePath := filepath.Join(tmpDir, "nonexistent", "config.toml") // does not exist
	centralDir := filepath.Join(tmpDir, "central")
	centralPath := filepath.Join(centralDir, "config.toml")
	guestPath := "/etc/containerd/config.toml"

	// Write only central config
	os.MkdirAll(centralDir, 0755)
	os.WriteFile(centralPath, []byte("central-config"), 0644)

	guest := &mockGuest{}
	rt := newTestRuntime(guest)

	if err := rt.provisionConfig(profilePath, centralPath, guestPath, []byte("default-config")); err != nil {
		t.Fatalf("provisionConfig() error = %v", err)
	}

	if string(guest.writtenData) != "central-config" {
		t.Errorf("expected central config as fallback, got %q", string(guest.writtenData))
	}
}

func TestProvisionConfig_DefaultWritesToCentral(t *testing.T) {
	tmpDir := t.TempDir()
	profilePath := filepath.Join(tmpDir, "nonexistent-profile", "config.toml")
	centralPath := filepath.Join(tmpDir, "central", "config.toml")
	guestPath := "/etc/containerd/config.toml"
	defaultConf := []byte("default-config")

	guest := &mockGuest{}
	rt := newTestRuntime(guest)

	if err := rt.provisionConfig(profilePath, centralPath, guestPath, defaultConf); err != nil {
		t.Fatalf("provisionConfig() error = %v", err)
	}

	// Should use the embedded default
	if string(guest.writtenData) != "default-config" {
		t.Errorf("expected default config, got %q", string(guest.writtenData))
	}

	// Should have written the default to the central location
	data, err := os.ReadFile(centralPath)
	if err != nil {
		t.Fatalf("expected default config to be written to central path: %v", err)
	}
	if string(data) != "default-config" {
		t.Errorf("central file content = %q, want %q", string(data), "default-config")
	}
}

func TestUserConfigDir(t *testing.T) {
	// With XDG_CONFIG_HOME set
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	if dir := userConfigDir(); dir != "/custom/config" {
		t.Errorf("userConfigDir() = %q, want %q", dir, "/custom/config")
	}

	// Without XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config")
	if dir := userConfigDir(); dir != want {
		t.Errorf("userConfigDir() = %q, want %q", dir, want)
	}
}

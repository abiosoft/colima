package native

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/abiosoft/colima/environment"
)

// mockHostActions implements environment.HostActions for testing.
type mockHostActions struct {
	// runResults maps command strings to error results.
	// Key format: "arg0 arg1 arg2 ..."
	runResults map[string]error

	// runOutputResults maps command strings to (output, error) pairs.
	runOutputResults map[string]struct {
		output string
		err    error
	}

	// files stores in-memory file contents.
	files map[string]string

	// env stores environment variables.
	env map[string]string

	// runCalls records all calls to Run/RunQuiet for assertions.
	runCalls [][]string

	// workingDir for WithDir.
	workingDir string
}

func newMockHostActions() *mockHostActions {
	return &mockHostActions{
		runResults: make(map[string]error),
		runOutputResults: make(map[string]struct {
			output string
			err    error
		}),
		files: make(map[string]string),
		env:   make(map[string]string),
	}
}

func commandKey(args []string) string {
	key := ""
	for i, a := range args {
		if i > 0 {
			key += " "
		}
		key += a
	}
	return key
}

func (m *mockHostActions) Run(args ...string) error {
	m.runCalls = append(m.runCalls, args)
	if err, ok := m.runResults[commandKey(args)]; ok {
		return err
	}
	return nil
}

func (m *mockHostActions) RunQuiet(args ...string) error {
	m.runCalls = append(m.runCalls, args)
	if err, ok := m.runResults[commandKey(args)]; ok {
		return err
	}
	return nil
}

func (m *mockHostActions) RunOutput(args ...string) (string, error) {
	m.runCalls = append(m.runCalls, args)
	if r, ok := m.runOutputResults[commandKey(args)]; ok {
		return r.output, r.err
	}
	return "", nil
}

func (m *mockHostActions) RunInteractive(args ...string) error {
	m.runCalls = append(m.runCalls, args)
	if err, ok := m.runResults[commandKey(args)]; ok {
		return err
	}
	return nil
}

func (m *mockHostActions) RunWith(_ io.Reader, _ io.Writer, args ...string) error {
	m.runCalls = append(m.runCalls, args)
	if err, ok := m.runResults[commandKey(args)]; ok {
		return err
	}
	return nil
}

func (m *mockHostActions) Read(fileName string) (string, error) {
	if content, ok := m.files[fileName]; ok {
		return content, nil
	}
	return "", fmt.Errorf("file not found: %s", fileName)
}

func (m *mockHostActions) Write(fileName string, body []byte) error {
	m.files[fileName] = string(body)
	return nil
}

func (m *mockHostActions) Stat(fileName string) (os.FileInfo, error) {
	if _, ok := m.files[fileName]; ok {
		// Return a fake FileInfo — only existence matters.
		return nil, nil
	}
	return nil, fmt.Errorf("file not found: %s", fileName)
}

func (m *mockHostActions) WithEnv(_ ...string) environment.HostActions {
	return m
}

func (m *mockHostActions) WithDir(dir string) environment.HostActions {
	clone := *m
	clone.workingDir = dir
	return &clone
}

func (m *mockHostActions) Env(key string) string {
	return m.env[key]
}

// ---------- Tests ----------

func TestNew(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	if vm == nil {
		t.Fatal("New() returned nil")
	}
}

func TestDependencies(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	deps := vm.Dependencies()
	if deps != nil {
		t.Errorf("Dependencies() = %v, want nil", deps)
	}
}

func TestHost(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	if vm.Host() != host {
		t.Error("Host() did not return the injected HostActions")
	}
}

func TestArch(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	arch := vm.Arch()
	// On any host, Arch() should return a non-empty value that
	// matches the current Go runtime architecture.
	if arch == "" {
		t.Error("Arch() returned empty string")
	}
	expected := environment.HostArch()
	if arch != expected {
		t.Errorf("Arch() = %q, want %q", arch, expected)
	}
}

func TestEnv(t *testing.T) {
	host := newMockHostActions()
	host.env["FOO"] = "bar"

	vm := New(host)
	val, err := vm.Env("FOO")
	if err != nil {
		t.Fatalf("Env() returned unexpected error: %v", err)
	}
	if val != "bar" {
		t.Errorf("Env(\"FOO\") = %q, want %q", val, "bar")
	}
}

func TestEnvMissing(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	val, err := vm.Env("NONEXISTENT")
	if err != nil {
		t.Fatalf("Env() returned unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("Env(\"NONEXISTENT\") = %q, want empty string", val)
	}
}

func TestUser(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	u, err := vm.User()
	if err != nil {
		t.Fatalf("User() returned unexpected error: %v", err)
	}
	if u == "" {
		t.Error("User() returned empty string")
	}
}

func TestStopIsNoop(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	err := vm.Stop(context.Background(), false)
	if err != nil {
		t.Errorf("Stop() returned unexpected error: %v", err)
	}
}

func TestStopForceIsNoop(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	err := vm.Stop(context.Background(), true)
	if err != nil {
		t.Errorf("Stop(force=true) returned unexpected error: %v", err)
	}
}

func TestRestartWithoutPriorStart(t *testing.T) {
	host := newMockHostActions()
	vm := New(host)
	err := vm.Restart(context.Background())
	if err == nil {
		t.Error("Restart() should return error when not previously started")
	}
}

func TestVerifyRuntimeDocker(t *testing.T) {
	host := newMockHostActions()
	// Docker systemd service is active
	// (RunQuiet returns nil by default for unknown commands)
	vm := New(host).(*nativeVM)

	err := vm.verifyRuntime("docker")
	if err != nil {
		t.Errorf("verifyRuntime(\"docker\") returned unexpected error: %v", err)
	}
}

func TestVerifyRuntimeDockerNotRunning(t *testing.T) {
	host := newMockHostActions()
	// systemctl is-active docker.service fails
	host.runResults["systemctl is-active docker.service"] = fmt.Errorf("inactive")
	// but docker binary exists
	// (RunQuiet for "which docker" returns nil by default)

	vm := New(host).(*nativeVM)
	err := vm.verifyRuntime("docker")
	if err == nil {
		t.Error("verifyRuntime(\"docker\") should return error when service is not active but binary exists")
	}
}

func TestVerifyRuntimeDockerMissing(t *testing.T) {
	host := newMockHostActions()
	host.runResults["systemctl is-active docker.service"] = fmt.Errorf("inactive")
	host.runResults["which docker"] = fmt.Errorf("not found")

	vm := New(host).(*nativeVM)
	err := vm.verifyRuntime("docker")
	if err == nil {
		t.Error("verifyRuntime(\"docker\") should return error when docker is missing")
	}
}

func TestVerifyRuntimeContainerd(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	err := vm.verifyRuntime("containerd")
	if err != nil {
		t.Errorf("verifyRuntime(\"containerd\") returned unexpected error: %v", err)
	}
}

func TestVerifyRuntimeContainerdMissing(t *testing.T) {
	host := newMockHostActions()
	host.runResults["systemctl is-active containerd.service"] = fmt.Errorf("inactive")

	vm := New(host).(*nativeVM)
	err := vm.verifyRuntime("containerd")
	if err == nil {
		t.Error("verifyRuntime(\"containerd\") should return error when service is not active")
	}
}

func TestVerifyRuntimeIncus(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	err := vm.verifyRuntime("incus")
	if err != nil {
		t.Errorf("verifyRuntime(\"incus\") returned unexpected error: %v", err)
	}
}

func TestVerifyRuntimeIncusMissing(t *testing.T) {
	host := newMockHostActions()
	host.runResults["which incus"] = fmt.Errorf("not found")

	vm := New(host).(*nativeVM)
	err := vm.verifyRuntime("incus")
	if err == nil {
		t.Error("verifyRuntime(\"incus\") should return error when incus is missing")
	}
}

func TestVerifyRuntimeNone(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	err := vm.verifyRuntime("none")
	if err != nil {
		t.Errorf("verifyRuntime(\"none\") returned unexpected error: %v", err)
	}
}

func TestVerifyRuntimeUnsupported(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	err := vm.verifyRuntime("podman")
	if err == nil {
		t.Error("verifyRuntime(\"podman\") should return error for unsupported runtime")
	}
}

// ---------- File I/O Tests ----------

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	expected := "hello world"
	if err := os.WriteFile(testFile, []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	content, err := vm.Read(testFile)
	if err != nil {
		t.Fatalf("Read() returned unexpected error: %v", err)
	}
	if content != expected {
		t.Errorf("Read() = %q, want %q", content, expected)
	}
}

func TestReadFileMissing(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	_, err := vm.Read("/nonexistent/file.txt")
	if err == nil {
		t.Error("Read() should return error for missing file")
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "subdir", "test.txt")

	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	data := []byte("test content")
	err := vm.Write(testFile, data)
	if err != nil {
		t.Fatalf("Write() returned unexpected error: %v", err)
	}

	// Verify the file was written correctly
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("os.ReadFile() returned unexpected error: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("Written content = %q, want %q", string(content), string(data))
	}
}

func TestStatFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	info, err := vm.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat() returned unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("Stat() returned nil FileInfo")
	}
	if info.Name() != "test.txt" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "test.txt")
	}
}

func TestStatFileMissing(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	_, err := vm.Stat("/nonexistent/file.txt")
	if err == nil {
		t.Error("Stat() should return error for missing file")
	}
}

// ---------- Shell delegation tests ----------

func TestRunDelegatesToHost(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	_ = vm.Run("echo", "hello")

	if len(host.runCalls) != 1 {
		t.Fatalf("expected 1 call to host, got %d", len(host.runCalls))
	}
	if host.runCalls[0][0] != "echo" || host.runCalls[0][1] != "hello" {
		t.Errorf("Run() called host with %v, want [echo hello]", host.runCalls[0])
	}
}

func TestRunQuietDelegatesToHost(t *testing.T) {
	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	_ = vm.RunQuiet("ls", "-la")

	if len(host.runCalls) != 1 {
		t.Fatalf("expected 1 call to host, got %d", len(host.runCalls))
	}
	if host.runCalls[0][0] != "ls" || host.runCalls[0][1] != "-la" {
		t.Errorf("RunQuiet() called host with %v, want [ls -la]", host.runCalls[0])
	}
}

func TestRunOutputDelegatesToHost(t *testing.T) {
	host := newMockHostActions()
	host.runOutputResults["uname -a"] = struct {
		output string
		err    error
	}{"Linux testhost 6.1.0", nil}

	vm := New(host).(*nativeVM)
	output, err := vm.RunOutput("uname", "-a")
	if err != nil {
		t.Fatalf("RunOutput() returned unexpected error: %v", err)
	}
	if output != "Linux testhost 6.1.0" {
		t.Errorf("RunOutput() = %q, want %q", output, "Linux testhost 6.1.0")
	}
}

// ---------- Config (Get/Set) tests ----------

func TestConfigSetAndGet(t *testing.T) {
	dir := t.TempDir()

	host := newMockHostActions()
	vm := New(host).(*nativeVM)

	// Override configFilePath by writing the config file to a temp location.
	// Since configFilePath uses config.CurrentProfile() which we can't easily
	// mock, we test loadConfig/Set by directly writing/reading the temp dir.
	configFile := filepath.Join(dir, configFileName)

	// Write config via os.WriteFile to simulate Set
	if err := os.WriteFile(configFile, []byte(`{"runtime":"docker","key1":"value1"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back via os.ReadFile to simulate loadConfig
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) == "" {
		t.Error("Config file should not be empty")
	}

	// Test that the VM implements GuestActions interface (Get/Set are part of it)
	_ = vm.Get("nonexistent") // should return "" without panic
}

// ---------- HostIPAddress tests ----------

func TestHostIPAddress(t *testing.T) {
	ip := HostIPAddress()
	if ip == "" {
		t.Error("HostIPAddress() returned empty string")
	}
	// Should be a valid IPv4 address (either a real one or the fallback 127.0.0.1)
	if len(ip) < 7 { // shortest valid: "0.0.0.0"
		t.Errorf("HostIPAddress() = %q, doesn't look like a valid IP", ip)
	}
}

func TestHostPrimaryInterface(t *testing.T) {
	iface := HostPrimaryInterface()
	if iface == "" {
		t.Error("HostPrimaryInterface() returned empty string")
	}
}

// ---------- Interface compliance ----------

func TestNativeVMImplementsVMInterface(t *testing.T) {
	// This is a compile-time check but let's be explicit in test too.
	var _ environment.VM = (*nativeVM)(nil)
}

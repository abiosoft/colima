package terminal

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestOutputNonTTY(t *testing.T) {
	// Test non-TTY output format
	var buf bytes.Buffer
	output := &Output{
		writer: &buf,
		isTTY:  false,
		done:   make(chan struct{}),
	}

	output.Begin("starting colima")
	output.Child("runtime docker")
	output.Begin("vm")
	output.Child("creating and starting")
	output.Done("done")

	got := buf.String()

	// Check expected lines
	expectedLines := []string{
		"... starting colima",
		"  ✓ runtime docker",
		"... vm",
		"  ✓ creating and starting",
		"✓ done",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(got, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, got)
		}
	}
}

func TestOutputTTY(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTY test in short mode")
	}

	// This test verifies basic TTY functionality
	var buf bytes.Buffer
	output := &Output{
		writer: &buf,
		isTTY:  true,
		done:   make(chan struct{}),
	}

	output.Start()

	output.Begin("starting")
	time.Sleep(50 * time.Millisecond) // Let spinner animate

	output.Child("step 1")
	output.Done("done")

	output.Stop()

	// In TTY mode, output should contain ANSI escape sequences for cursor control
	got := buf.String()
	if !strings.Contains(got, "\033[") {
		t.Errorf("expected TTY output to contain ANSI escape sequences")
	}

	// Should contain checkmark symbol
	if !strings.Contains(got, "✓") {
		t.Errorf("expected output to contain checkmark symbol")
	}
}

func TestVerboseWriterCoordination(t *testing.T) {
	// Test that VerboseWriter coordinates with Output
	output := &Output{
		writer: &bytes.Buffer{},
		isTTY:  true,
		done:   make(chan struct{}),
	}
	output.Start()
	defer output.Stop()

	// Initially external should not be active
	output.mu.Lock()
	if output.externalActive {
		t.Error("expected externalActive to be false initially")
	}
	output.mu.Unlock()

	// Create verbose writer and write to it
	vw := output.VerboseWriter(3)
	_, _ = vw.Write([]byte("test line\n"))

	// After write, external should be active
	output.mu.Lock()
	if !output.externalActive {
		t.Error("expected externalActive to be true after verbose write")
	}
	output.mu.Unlock()

	// Close the writer
	_ = vw.Close()

	// After close, external should not be active
	output.mu.Lock()
	if output.externalActive {
		t.Error("expected externalActive to be false after verbose close")
	}
	output.mu.Unlock()
}

func TestProgressWriterCoordination(t *testing.T) {
	// Test that ProgressWriter coordinates with Output
	output := &Output{
		writer: &bytes.Buffer{},
		isTTY:  true,
		done:   make(chan struct{}),
	}
	output.Start()
	defer output.Stop()

	pw := output.ProgressWriter()

	// Before Begin, external should not be active
	output.mu.Lock()
	if output.externalActive {
		t.Error("expected externalActive to be false before progress Begin")
	}
	output.mu.Unlock()

	// Begin progress
	pw.Begin()

	output.mu.Lock()
	if !output.externalActive {
		t.Error("expected externalActive to be true after progress Begin")
	}
	output.mu.Unlock()

	// End progress
	pw.End()

	output.mu.Lock()
	if output.externalActive {
		t.Error("expected externalActive to be false after progress End")
	}
	output.mu.Unlock()
}

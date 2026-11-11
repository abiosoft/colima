package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestCountDisplayLines(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		termWidth int
		want      int
	}{
		{name: "short line", line: "> short", termWidth: 80, want: 1},
		{name: "wrapped line", line: "> 12345678901", termWidth: 10, want: 2},
		{name: "exact width", line: "> 12345678", termWidth: 10, want: 1},
		{name: "carriage return ignored", line: "> abc\rdef", termWidth: 80, want: 1},
		{name: "ansi ignored", line: "> \x1b[90mabcdef\x1b[0m", termWidth: 4, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countDisplayLines(tt.line, tt.termWidth); got != tt.want {
				t.Fatalf("countDisplayLines(%q, %d) = %d, want %d", tt.line, tt.termWidth, got, tt.want)
			}
		})
	}
}

func TestNormalizeDisplayText(t *testing.T) {
	input := "\x1b[90mhello\r\n\x07world\x1b[0m"
	if got, want := normalizeDisplayText(input), "helloworld"; got != want {
		t.Fatalf("normalizeDisplayText(%q) = %q, want %q", input, got, want)
	}
}

func TestDiagnosticsStayOutsideProgressArea(t *testing.T) {
	for _, line := range []string{
		"time=\"2026-11-11T00:00:00Z\" level=warning msg=\"check the dependency setting\"",
		"time=\"2026-11-11T00:00:00Z\" level=error msg=\"dependency failed\"",
		"WARN[0000] check the dependency setting",
	} {
		t.Run(line, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			stderr := os.Stderr
			os.Stderr = file
			defer func() { os.Stderr = stderr }()

			writer := &verboseWriter{lineHeight: 2, lines: []string{"progress"}}
			writer.buf.WriteString(line)
			writer.addLine()

			if len(writer.lines) != 1 || writer.lines[0] != "progress" {
				t.Fatalf("diagnostic entered temporary progress area: %q", writer.lines)
			}
			output, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(output), writer.sanitizeLine(line)) {
				t.Fatalf("diagnostic was not printed: %q", output)
			}
			if writer.buf.Len() != 0 {
				t.Fatal("diagnostic buffer was not cleared")
			}
		})
	}
}

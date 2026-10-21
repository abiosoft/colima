package terminal

import "testing"

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
		{name: "wide characters wrap by terminal columns", line: "> 界界界界界", termWidth: 10, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countDisplayLines(tt.line, tt.termWidth); got != tt.want {
				t.Fatalf("countDisplayLines(%q, %d) = %d, want %d", tt.line, tt.termWidth, got, tt.want)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		termWidth int
		want      int
	}{
		{name: "exact width does not add a wrapped line", text: "1234567890", termWidth: 10, want: 1},
		{name: "overflow wraps", text: "12345678901", termWidth: 10, want: 2},
		{name: "newline separates lines", text: "first\nsecond", termWidth: 80, want: 2},
		{name: "wide characters use terminal columns", text: "界界界界界", termWidth: 10, want: 1},
		{name: "wide character overflow wraps", text: "界界界界界a", termWidth: 10, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countLines(tt.text, tt.termWidth); got != tt.want {
				t.Fatalf("countLines(%q, %d) = %d, want %d", tt.text, tt.termWidth, got, tt.want)
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

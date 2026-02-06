package terminal

import (
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/term"
)

var isTerminal = term.IsTerminal(int(os.Stdout.Fd()))

// ClearLine clears the previous line of the terminal
func ClearLine() {
	if !isTerminal {
		return
	}

	fmt.Print("\033[1A \033[2K \r")
}

// ClearLines clears the specified number of lines from the terminal.
func ClearLines(count int) {
	if !isTerminal {
		return
	}

	for i := 0; i < count; i++ {
		ClearLine()
	}
}

// InteractiveIO tracks output lines for interactive commands.
// Use Clear() after the command completes to clear all tracked lines.
type InteractiveIO struct {
	lines int
	mu    sync.Mutex
}

// NewInteractiveIO creates a new interactive IO tracker.
func NewInteractiveIO() *InteractiveIO {
	return &InteractiveIO{}
}

// Writer returns an io.Writer that tracks output lines.
func (i *InteractiveIO) Writer(w io.Writer) io.Writer {
	return &interactiveWriter{io: i, w: w}
}

// Lines returns the total number of lines tracked.
func (i *InteractiveIO) Lines() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.lines
}

// Clear clears all tracked lines from the terminal.
func (i *InteractiveIO) Clear() {
	i.mu.Lock()
	defer i.mu.Unlock()
	ClearLines(i.lines)
	i.lines = 0
}

func (i *InteractiveIO) addLines(n int) {
	i.mu.Lock()
	i.lines += n
	i.mu.Unlock()
}

// interactiveWriter wraps a writer and counts newlines.
type interactiveWriter struct {
	io *InteractiveIO
	w  io.Writer
}

func (w *interactiveWriter) Write(p []byte) (n int, err error) {
	count := 0
	for _, b := range p {
		if b == '\n' {
			count++
		}
	}
	if count > 0 {
		w.io.addLines(count)
	}
	return w.w.Write(p)
}

package terminal

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/term"
)

var isTerminal = term.IsTerminal(int(os.Stdout.Fd()))

// IsTerminal returns true if stdout is a terminal.
func IsTerminal() bool {
	return isTerminal
}

// ClearLine clears the previous line of the terminal
func ClearLine() {
	if !isTerminal {
		return
	}

	fmt.Print("\033[1A \033[2K \r")
}

// EnterAltScreen switches to the alternate screen buffer.
// This preserves the main terminal content which can be restored
// by calling ExitAltScreen.
func EnterAltScreen() {
	if !isTerminal {
		return
	}
	// Switch to alternate screen buffer and move cursor to top-left
	fmt.Print("\033[?1049h\033[H")
}

// ExitAltScreen switches back to the main screen buffer,
// restoring the previous terminal content.
func ExitAltScreen() {
	if !isTerminal {
		return
	}
	fmt.Print("\033[?1049l")
}

// WithAltScreen runs the provided function in the alternate screen buffer.
// The main terminal content is preserved and restored after the function completes.
// Handles Ctrl-C to ensure the terminal is restored even on interrupt.
//
// If header lines are provided, they are joined with newlines and displayed as a
// fixed header at the top of the screen. The command output scrolls below the header.
// The number of header lines is computed automatically based on newlines and terminal width.
func WithAltScreen(fn func() error, header ...string) error {
	hasHeader := len(header) > 0
	var headerText string
	if hasHeader {
		headerText = strings.Join(header, "\n")
	}

	if !isTerminal {
		if hasHeader {
			fmt.Println(headerText)
		}
		return fn()
	}

	EnterAltScreen()

	// Handle Ctrl-C to ensure terminal is restored even on interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			if hasHeader {
				fmt.Print("\033[r") // Reset scroll region
			}
			ExitAltScreen()
			os.Exit(1)
		case <-done:
			return
		}
	}()

	if hasHeader {
		// Get terminal dimensions
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			width = 80
			height = 24
		}

		// Print the header
		fmt.Println(headerText)

		// Calculate number of lines used by the header
		headerLines := countLines(headerText, width) + 1 // +1 for padding

		// Set scroll region from headerLines+1 to bottom
		// This keeps the header fixed while everything below scrolls
		fmt.Printf("\033[%d;%dr", headerLines+1, height)

		// Move cursor to the first line of the scroll region
		fmt.Printf("\033[%d;1H", headerLines+1)
	}

	err := fn()

	if hasHeader {
		// Reset scroll region
		fmt.Print("\033[r")
	}

	close(done)
	ExitAltScreen()

	return err
}

// countLines calculates the number of terminal lines a string will occupy,
// accounting for newlines and line wrapping based on terminal width.
func countLines(s string, termWidth int) int {
	if termWidth <= 0 {
		termWidth = 80
	}

	lines := 1
	currentLineLen := 0

	for _, ch := range s {
		if ch == '\n' {
			lines++
			currentLineLen = 0
		} else {
			currentLineLen++
			if currentLineLen >= termWidth {
				lines++
				currentLineLen = 0
			}
		}
	}

	return lines
}

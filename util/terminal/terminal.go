package terminal

import (
	"fmt"
	"os"
	"os/signal"

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
func WithAltScreen(fn func() error) error {
	if !isTerminal {
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
			ExitAltScreen()
			os.Exit(1)
		case <-done:
			return
		}
	}()

	err := fn()

	close(done)
	ExitAltScreen()

	return err
}

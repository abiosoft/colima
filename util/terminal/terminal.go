package terminal

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

var isTerminal = terminal.IsTerminal(int(os.Stdout.Fd()))

// ClearLine clears the previous line of the terminal
func ClearLine() {
	if !isTerminal {
		return
	}

	fmt.Print("\033[1A \033[2K \r")
}

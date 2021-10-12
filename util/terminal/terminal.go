package terminal

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

// ClearLine clears the previous line of the terminal
var isTerminal = terminal.IsTerminal(int(os.Stdout.Fd()))

func ClearLine() {
	if !isTerminal {
		return
	}

	fmt.Print("\033[1A \033[2K \r")
}

package host

import (
	"bytes"
	"fmt"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"time"
)

var _ io.WriteCloser = (*verboseOutput)(nil)

type verboseOutput struct {
	buf   bytes.Buffer
	lines []string

	lineHeight int
	termWidth  int

	lastUpdate time.Time
}

func newVerboseOutput(lineHeight int) io.WriteCloser {
	return &verboseOutput{lineHeight: lineHeight}
}

func (v *verboseOutput) Write(p []byte) (n int, err error) {
	for i, c := range p {
		if c != '\n' {
			v.buf.WriteByte(c)
			continue
		}

		if err := v.refresh(); err != nil {
			return i + 1, err
		}

	}
	return len(p), nil
}

func (v *verboseOutput) refresh() error {
	v.clearScreen()
	v.addLine()
	return v.printScreen()
}

func (v *verboseOutput) addLine() {
	if len(v.lines) >= v.lineHeight {
		v.lines = v.lines[1:]
	}
	v.lines = append(v.lines, v.buf.String())
	v.buf.Truncate(0)
}

func (v *verboseOutput) Close() error {
	if v.buf.Len() > 0 {
		if err := v.refresh(); err != nil {
			return err
		}
	}

	v.clearScreen()
	return nil
}

// prepareScreen creates whitespace for upcoming contents.
func (v *verboseOutput) prepareScreen() {
	for i := 0; i < v.lineHeight; i++ {
		fmt.Println()
	}
}

func (v *verboseOutput) printScreen() error {
	if err := v.updateTerm(); err != nil {
		return err
	}

	for _, line := range v.lines {
		if len(line) > v.termWidth {
			line = line[:v.termWidth]
		}
		line = color.HiBlackString(line)
		fmt.Println(line)
	}
	return nil
}

func (v verboseOutput) clearScreen() {
	for range v.lines {
		fmt.Print("\033[1A \033[2K \r")
	}
}

func (v *verboseOutput) updateTerm() error {
	// no need to refresh so quickly
	if time.Now().Sub(v.lastUpdate) < time.Second*2 {
		return nil
	}

	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("error getting terminal size: %w", err)
	}
	v.termWidth = w

	return nil
}

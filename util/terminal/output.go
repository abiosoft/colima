package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var _ io.WriteCloser = (*verboseWriter)(nil)

type verboseWriter struct {
	buf   bytes.Buffer
	lines []string

	lineHeight int
	termWidth  int
	overflow   int

	lastUpdate time.Time
}

// NewVerboseWriter creates a new verbose writer.
// A verbose writer pipes the input received to the stdout while tailing the specified lines.
// Calling `Close` when done is recommended to clear the last uncleared output.
func NewVerboseWriter(lineHeight int) io.WriteCloser {
	return &verboseWriter{lineHeight: lineHeight}
}

func (v *verboseWriter) Write(p []byte) (n int, err error) {
	// if it's not a terminal, simply write to stdout
	if !isTerminal {
		return os.Stdout.Write(p)
	}

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

func (v *verboseWriter) printLineVerbose() {
	line := v.sanitizeLine(v.buf.String())
	line = color.HiBlackString(line)
	_, _ = fmt.Fprintln(os.Stderr, line)
}

func (v *verboseWriter) refresh() error {
	v.clearScreen()
	v.addLine()
	return v.printScreen()
}

func (v *verboseWriter) addLine() {
	defer v.buf.Truncate(0)

	// if height <=0, do not scroll
	if v.lineHeight <= 0 {
		v.printLineVerbose()
		return
	}

	if len(v.lines) >= v.lineHeight {
		v.lines = v.lines[1:]
	}
	v.lines = append(v.lines, v.buf.String())
}

func (v *verboseWriter) Close() error {
	if v.buf.Len() > 0 {
		if err := v.refresh(); err != nil {
			return err
		}
	}

	v.clearScreen()
	return nil
}

func (v verboseWriter) sanitizeLine(line string) string {
	// remove logrus noises
	if strings.HasPrefix(line, "time=") && strings.Contains(line, "msg=") {
		line = line[strings.Index(line, "msg=")+4:]
		if l, err := strconv.Unquote(line); err == nil {
			line = l
		}
	}

	return "> " + line
}

func (v *verboseWriter) printScreen() error {
	if err := v.updateTerm(); err != nil {
		return err
	}

	v.overflow = 0
	for _, line := range v.lines {
		line = v.sanitizeLine(line)
		if len(line) > v.termWidth {
			v.overflow += len(line) / v.termWidth
		}
		line = color.HiBlackString(line)
		fmt.Println(line)
	}
	return nil
}

func (v verboseWriter) clearScreen() {
	for i := 0; i < len(v.lines)+v.overflow; i++ {
		ClearLine()
	}
}

func (v *verboseWriter) updateTerm() error {
	// no need to refresh so quickly
	if time.Since(v.lastUpdate) < time.Second*2 {
		return nil
	}
	v.lastUpdate = time.Now().UTC()

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("error getting terminal size: %w", err)
	}
	// A width of zero would result in a division by zero panic when computing overflow
	// in printScreen. Therefore, set it to a safe - even though probably wrong - value.
	// We use <= 0 here because negative values are guaranteed to lead to unexpected
	// results, even if they don't cause panics.
	if w <= 0 {
		w = 80
	}
	v.termWidth = w

	return nil
}

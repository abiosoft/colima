package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// spinnerFrames for animation (braille spinner - circular pattern)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	symbolDone  = "✓"
	symbolError = "✗"
)

// ctxKeyOutput is the context key for Output
type ctxKeyOutput struct{}

// CtxKeyOutput is the context key to retrieve Output from context
var CtxKeyOutput = ctxKeyOutput{}

// OutputFromContext retrieves Output from context, returns nil if not present
func OutputFromContext(ctx context.Context) *Output {
	if o, ok := ctx.Value(CtxKeyOutput).(*Output); ok {
		return o
	}
	return nil
}

// LineState represents the state of a status line
type LineState int

const (
	StateRunning LineState = iota
	StateDone
	StateError
)

// statusLine represents a single line in the status output
type statusLine struct {
	text     string
	state    LineState
	indent   int
	children []*statusLine
}

// Output manages all terminal output with proper coordination.
// It owns the terminal and provides writers to components.
// All cursor manipulation is handled internally - components just write.
type Output struct {
	mu sync.Mutex

	// status section
	lines        []*statusLine
	current      *statusLine
	spinnerIndex int

	// animation
	ticker  *time.Ticker
	done    chan struct{}
	started bool

	// rendering state
	renderedLines int
	writer        io.Writer
	isTTY         bool

	// external output tracking (verbose/progress)
	externalActive bool
}

// NewOutput creates a new terminal output manager
func NewOutput() *Output {
	return &Output{
		writer: os.Stderr,
		isTTY:  isTerminal,
		done:   make(chan struct{}),
	}
}

// Start begins the output manager and spinner animation
func (o *Output) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.started {
		return
	}
	o.started = true

	if !o.isTTY {
		return
	}

	o.ticker = time.NewTicker(100 * time.Millisecond)
	go o.animate()
}

// Stop stops the output manager
func (o *Output) Stop() {
	o.mu.Lock()
	if !o.started {
		o.mu.Unlock()
		return
	}

	if o.ticker != nil {
		o.ticker.Stop()
		select {
		case <-o.done:
		default:
			close(o.done)
		}
	}

	if o.isTTY {
		o.render()
	}
	o.mu.Unlock()
}

// animate runs the spinner animation loop
func (o *Output) animate() {
	for {
		select {
		case <-o.done:
			return
		case <-o.ticker.C:
			o.mu.Lock()
			if !o.externalActive {
				o.spinnerIndex = (o.spinnerIndex + 1) % len(spinnerFrames)
				o.render()
			}
			o.mu.Unlock()
		}
	}
}

// Begin starts a new context with a spinner
func (o *Output) Begin(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// complete previous context if any
	if o.current != nil {
		o.current.state = StateDone
	}

	line := &statusLine{
		text:  text,
		state: StateRunning,
	}
	o.lines = append(o.lines, line)
	o.current = line

	if !o.isTTY {
		o.printPlain(line)
		return
	}
	o.render()
}

// Child adds a completed child item under the current context
func (o *Output) Child(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	child := &statusLine{
		text:   text,
		state:  StateDone,
		indent: 1,
	}

	if o.current != nil {
		o.current.children = append(o.current.children, child)
	} else {
		o.lines = append(o.lines, child)
	}

	if !o.isTTY {
		o.printPlain(child)
		return
	}
	o.render()
}

// Done adds a final "done" message
func (o *Output) Done(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.current != nil {
		o.current.state = StateDone
	}

	line := &statusLine{
		text:  text,
		state: StateDone,
	}
	o.lines = append(o.lines, line)
	o.current = nil

	if !o.isTTY {
		o.printPlain(line)
		return
	}
	o.render()
}

// Error adds an error message
func (o *Output) Error(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.current != nil {
		o.current.state = StateError
	}

	line := &statusLine{
		text:  text,
		state: StateError,
	}
	o.lines = append(o.lines, line)
	o.current = nil

	if !o.isTTY {
		o.printPlain(line)
		return
	}
	o.render()
}

// render draws all status lines (must be called with lock held)
func (o *Output) render() {
	// clear previous output
	for range o.renderedLines {
		fmt.Fprint(o.writer, "\033[1A\033[2K")
	}

	// render all lines
	o.renderedLines = 0
	for _, line := range o.lines {
		o.renderLine(line)
		o.renderedLines++
		for _, child := range line.children {
			o.renderLine(child)
			o.renderedLines++
		}
	}
}

// renderLine renders a single status line
func (o *Output) renderLine(line *statusLine) {
	indent := strings.Repeat("  ", line.indent)
	symbol := o.getSymbol(line.state)

	var formatted string
	switch line.state {
	case StateRunning:
		formatted = color.CyanString(symbol) + " " + line.text
	case StateDone:
		formatted = color.GreenString(symbol) + " " + line.text
	case StateError:
		formatted = color.RedString(symbol) + " " + line.text
	default:
		formatted = symbol + " " + line.text
	}

	fmt.Fprintln(o.writer, indent+formatted)
}

// getSymbol returns the appropriate symbol for the state
func (o *Output) getSymbol(state LineState) string {
	switch state {
	case StateRunning:
		return spinnerFrames[o.spinnerIndex]
	case StateDone:
		return symbolDone
	case StateError:
		return symbolError
	default:
		return " "
	}
}

// printPlain prints a line in plain format for non-TTY output
func (o *Output) printPlain(line *statusLine) {
	indent := strings.Repeat("  ", line.indent)
	var prefix string
	switch line.state {
	case StateRunning:
		prefix = "..."
	case StateDone:
		prefix = symbolDone
	case StateError:
		prefix = symbolError
	default:
		prefix = " "
	}
	fmt.Fprintf(o.writer, "%s%s %s\n", indent, prefix, line.text)
}

// IsTTY returns whether output is to a terminal
func (o *Output) IsTTY() bool {
	return o.isTTY
}

// VerboseWriter returns a writer for verbose subcommand output.
// The writer automatically coordinates with the status display.
// lineHeight controls scrolling (-1 to disable scrolling and print all lines).
func (o *Output) VerboseWriter(lineHeight int) io.WriteCloser {
	return &verboseWriter{
		output:     o,
		lineHeight: lineHeight,
	}
}

// verboseWriter is an io.WriteCloser that handles verbose subcommand output
// with automatic coordination with the parent Output.
type verboseWriter struct {
	output *Output

	buf   bytes.Buffer
	lines []string

	lineHeight int
	termWidth  int
	overflow   int

	lastUpdate time.Time
	active     bool

	mu sync.Mutex
}

func (v *verboseWriter) Write(p []byte) (n int, err error) {
	// if not a terminal, just write to stdout
	if v.output == nil || !v.output.isTTY {
		return os.Stdout.Write(p)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// mark external output as active on first write
	if !v.active {
		v.output.mu.Lock()
		v.output.externalActive = true
		v.output.mu.Unlock()
		v.active = true
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

func (v *verboseWriter) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.buf.Len() > 0 {
		if err := v.refresh(); err != nil {
			return err
		}
	}

	v.clearScreen()

	// mark external output as inactive
	if v.active && v.output != nil {
		v.output.mu.Lock()
		v.output.externalActive = false
		v.output.mu.Unlock()
		v.active = false
	}

	return nil
}

func (v *verboseWriter) refresh() error {
	v.clearScreen()
	v.addLine()
	return v.printScreen()
}

func (v *verboseWriter) addLine() {
	defer v.buf.Reset()

	// if height <= 0, do not scroll
	if v.lineHeight <= 0 {
		v.printLineVerbose()
		return
	}

	if len(v.lines) >= v.lineHeight {
		v.lines = v.lines[1:]
	}
	v.lines = append(v.lines, v.buf.String())
}

func (v *verboseWriter) printLineVerbose() {
	line := v.sanitizeLine(v.buf.String())
	line = color.HiBlackString(line)
	_, _ = fmt.Fprintln(os.Stderr, line)
}

func (v *verboseWriter) sanitizeLine(line string) string {
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
			if len(line)%v.termWidth == 0 {
				v.overflow -= 1
			}
		}
		line = color.HiBlackString(line)
		fmt.Println(line)
	}
	return nil
}

func (v *verboseWriter) clearScreen() {
	for i := 0; i < len(v.lines)+v.overflow; i++ {
		ClearLine()
	}
}

func (v *verboseWriter) updateTerm() error {
	if time.Since(v.lastUpdate) < time.Second*2 {
		return nil
	}
	v.lastUpdate = time.Now().UTC()

	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("error getting terminal size: %w", err)
	}
	if w <= 0 {
		w = 80
	}
	v.termWidth = w
	return nil
}

// ProgressWriter returns a writer wrapper that coordinates progress output
// (like download bars) with the status display.
func (o *Output) ProgressWriter() *ProgressWriter {
	return &ProgressWriter{output: o}
}

// ProgressWriter coordinates progress bar output with the status display
type ProgressWriter struct {
	output *Output
	active bool
}

// Begin should be called before starting progress output
func (p *ProgressWriter) Begin() {
	if p.output == nil {
		return
	}
	p.output.mu.Lock()
	p.output.externalActive = true
	p.output.mu.Unlock()
	p.active = true
}

// End should be called after progress output is complete
func (p *ProgressWriter) End() {
	if p.output == nil || !p.active {
		return
	}
	p.output.mu.Lock()
	p.output.externalActive = false
	p.output.mu.Unlock()
	p.active = false
}

// NewVerboseWriter creates a standalone verbose writer (for backward compatibility).
// Prefer using Output.VerboseWriter() when an Output instance is available.
func NewVerboseWriter(lineHeight int) io.WriteCloser {
	return &verboseWriter{
		lineHeight: lineHeight,
	}
}

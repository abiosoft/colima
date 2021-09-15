package runner

import (
	"fmt"
	"github.com/abiosoft/colima/log"
)

// New creates a new runner instance.
func New(name string) Instance {
	return &namedInstance{
		name: name,
	}
}

// Instance is a runner instance.
type Instance interface {
	// Init initiates a new runner using the current instance.
	Init() *Runner
	// Logger returns the instance logger.
	Logger() *log.Logger
}

var _ Instance = (*namedInstance)(nil)

type namedInstance struct {
	name string
	log  *log.Logger
}

func (n namedInstance) Logger() *log.Logger {
	if n.log == nil {
		n.log = log.New(n.name)
	}
	return n.log
}

func (n namedInstance) Init() *Runner {
	return &Runner{
		Logger: n.Logger(),
	}
}

// Runner is function runner. Functions are chained and
// executed in order.
type Runner struct {
	funcs     []func() error
	lastStage string
	*log.Logger
}

// Add adds a new function to the runner.
func (r *Runner) Add(f func() error) {
	r.funcs = append(r.funcs, f)
}

// Stage sets the current stage of the runner.
func (r *Runner) Stage(s string) {
	r.Println(s, "...")
	r.lastStage = s
}

// Stagef is like stage with string format.
func (r *Runner) Stagef(format string, s ...interface{}) {
	f := fmt.Sprintf(format, s...)
	r.Stage(f)
}

// Run runs the command chain.
// The first errored function terminates the chain and the
// error is returned. Otherwise, returns nil.
func (r Runner) Run() error {
	for _, f := range r.funcs {
		if f == nil {
			continue
		}

		err := f()
		if err == nil {
			continue
		}

		if r.lastStage == "" {
			return err
		}
		return fmt.Errorf("error at '%s': %w", r.lastStage, err)
	}
	return nil
}

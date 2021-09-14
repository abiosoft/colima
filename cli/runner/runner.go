package runner

import (
	"fmt"
	"github.com/abiosoft/colima/clog"
)

// Func is a runner function.
type Func func() error

// New creates a new runner.
func New(name string, funcs ...Func) *Runner {
	return &Runner{
		name:   name,
		funcs:  funcs,
		Logger: clog.For(name),
	}
}

// Runner is function runner. Functions are chained and
// executed in order.
type Runner struct {
	name      string
	funcs     []Func
	lastStage string
	*clog.Logger
}

// Add adds a new function to the runner.
func (r *Runner) Add(f func() error) {
	r.funcs = append(r.funcs, f)
}

// Stage sets the current stage of the runner.
func (r Runner) Stage(s string) {
	r.Println(s, "...")
	r.lastStage = s
}

// Stagef is like stage with string format.
func (r Runner) Stagef(format string, s ...interface{}) {
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

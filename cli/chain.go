package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
)

// CtxKeyQuiet is the context key to mute the chain.
var CtxKeyQuiet = struct{ key string }{key: "quiet"}

// errNonFatal is a non fatal error
type errNonFatal struct {
	err error
}

// Error implements error
func (e errNonFatal) Error() string { return e.err.Error() }

// ErrNonFatal creates a non-fatal error for a command chain.
// A warning would be printed instead of terminating the chain.
func ErrNonFatal(err error) error {
	return errNonFatal{err}
}

// New creates a new runner instance.
func New(name string) CommandChain {
	return &namedCommandChain{
		name: name,
	}
}

type cFunc struct {
	f func() error
	s string
}

// CommandChain is a chain of commands.
// commands are executed in order.
type CommandChain interface {
	// Init initiates a new runner using the current instance.
	Init(ctx context.Context) *ActiveCommandChain
	// Logger returns the instance logger.
	Logger(ctx context.Context) *log.Entry
}

var _ CommandChain = (*namedCommandChain)(nil)

type namedCommandChain struct {
	name string
	log  *log.Entry
}

func (n namedCommandChain) Logger(ctx context.Context) *log.Entry {
	if quiet, _ := ctx.Value(CtxKeyQuiet).(bool); quiet {
		l := log.New()
		l.SetOutput(io.Discard)
		return l.WithContext(ctx)
	}
	if n.log == nil {
		n.log = log.WithField("context", n.name).WithContext(ctx)
	}
	return n.log
}

func (n namedCommandChain) Init(ctx context.Context) *ActiveCommandChain {
	return &ActiveCommandChain{
		log: n.Logger(ctx),
	}
}

// ActiveCommandChain is an active command chain.
type ActiveCommandChain struct {
	funcs     []cFunc
	lastStage string
	log       *log.Entry

	executing bool
}

// Logger returns the logger for the command chain.
func (a *ActiveCommandChain) Logger() *log.Entry { return a.log }

// Add adds a new function to the runner.
func (a *ActiveCommandChain) Add(f func() error) {
	a.funcs = append(a.funcs, cFunc{f: f})
}

// Stage sets the current stage of the runner.
func (a *ActiveCommandChain) Stage(s string) {
	if a.executing {
		a.log.Println(s, "...")
		return
	}
	a.funcs = append(a.funcs, cFunc{s: s})
}

// Stagef is like stage with string format.
func (a *ActiveCommandChain) Stagef(format string, s ...interface{}) {
	f := fmt.Sprintf(format, s...)
	a.Stage(f)
}

// Exec executes the command chain.
// The first errored function terminates the chain and the
// error is returned. Otherwise, returns nil.
func (a *ActiveCommandChain) Exec() error {
	a.executing = true
	defer func() { a.executing = false }()

	for _, f := range a.funcs {
		if f.f == nil {
			if f.s != "" {
				a.log.Println(f.s, "...")
				a.lastStage = f.s
			}
			continue
		}

		// success
		err := f.f()
		if err == nil {
			continue
		}

		// warning
		if _, ok := err.(errNonFatal); ok {
			if a.lastStage == "" {
				a.log.Warnln(err)
			} else {
				a.log.Warnln(fmt.Errorf("error at '%s': %w", a.lastStage, err))
			}
			continue
		}

		// error
		if a.lastStage == "" {
			return err
		}
		return fmt.Errorf("error at '%s': %w", a.lastStage, err)
	}
	return nil
}

// Retry retries `f` up to `count` times at interval.
// If after `count` attempts there is an error, the command chain is terminated with the final error.
// retryCount starts from 1.
func (a *ActiveCommandChain) Retry(stage string, interval time.Duration, count int, f func(retryCount int) error) {
	a.Add(func() (err error) {
		var i int
		for err = f(i + 1); i < count && err != nil; i, err = i+1, f(i+1) {
			if stage != "" {
				a.log.Println(stage, "...")
			}
			time.Sleep(interval)
		}
		return err
	})
}

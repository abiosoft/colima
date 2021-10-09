package cli

import (
	"fmt"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

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
	Init() *ActiveCommandChain
	// Logger returns the instance logger.
	Logger() *logrus.Entry
}

var _ CommandChain = (*namedCommandChain)(nil)

type namedCommandChain struct {
	name string
	log  *logrus.Entry
}

func (n namedCommandChain) Logger() *logrus.Entry {
	if n.log == nil {
		n.log = util.Logger().WithField("context", n.name)
	}
	return n.log
}

func (n namedCommandChain) Init() *ActiveCommandChain {
	return &ActiveCommandChain{
		Entry: n.Logger(),
	}
}

// ActiveCommandChain is an active command chain.
type ActiveCommandChain struct {
	funcs     []cFunc
	lastStage string
	*logrus.Entry
}

// Add adds a new function to the runner.
func (a *ActiveCommandChain) Add(f func() error) {
	a.funcs = append(a.funcs, cFunc{f: f})
}

// Stage sets the current stage of the runner.
func (a *ActiveCommandChain) Stage(s string) {
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
func (a ActiveCommandChain) Exec() error {
	for _, f := range a.funcs {
		if f.f == nil {
			if f.s != "" {
				a.Println(f.s, "...")
				a.lastStage = f.s
			}
			continue
		}

		err := f.f()
		if err == nil {
			continue
		}

		if a.lastStage == "" {
			return err
		}
		return fmt.Errorf("error at '%s': %w", a.lastStage, err)
	}
	return nil
}

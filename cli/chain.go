package cli

import (
	"fmt"
	"github.com/abiosoft/colima/log"
)

// New creates a new runner instance.
func New(name string) CommandChain {
	return &namedInstance{
		name: name,
	}
}

// CommandChain is a chain of commands.
// commands are executed in order.
type CommandChain interface {
	// Init initiates a new runner using the current instance.
	Init() *ActiveCommandChain
	// Logger returns the instance logger.
	Logger() *log.Logger
}

var _ CommandChain = (*namedInstance)(nil)

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

func (n namedInstance) Init() *ActiveCommandChain {
	return &ActiveCommandChain{
		Logger: n.Logger(),
	}
}

// ActiveCommandChain is an active command chain.
type ActiveCommandChain struct {
	funcs     []func() error
	lastStage string
	*log.Logger
}

// Add adds a new function to the runner.
func (r *ActiveCommandChain) Add(f func() error) {
	r.funcs = append(r.funcs, f)
}

// Stage sets the current stage of the runner.
func (r *ActiveCommandChain) Stage(s string) {
	r.Println(s, "...")
	r.lastStage = s
}

// Stagef is like stage with string format.
func (r *ActiveCommandChain) Stagef(format string, s ...interface{}) {
	f := fmt.Sprintf(format, s...)
	r.Stage(f)
}

// Exec executes the command chain.
// The first errored function terminates the chain and the
// error is returned. Otherwise, returns nil.
func (r ActiveCommandChain) Exec() error {
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

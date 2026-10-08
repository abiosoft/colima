package inotify

import (
	"context"
	"fmt"

	"github.com/abiosoft/colima/util"
	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
)

const eventBufferSize = 1024

type dirWatcher interface {
	// Watch watches directories recursively for changes and sends message via c on
	// modifications to files within the watched directories.
	//
	// Watch returns immediately and runs the watcher in the background.
	// An error is returned when the watcher can not be started in background.
	//
	// The watcher terminates on fatal error or when ctx is done.
	Watch(ctx context.Context, dirs []string, c chan<- string) error
}

type defaultWatcher struct {
	log *logrus.Entry
}

// Watch implements dirWatcher
func (d *defaultWatcher) Watch(ctx context.Context, dirs []string, mod chan<- string) error {
	log := d.log
	// notify drops events when the channel is full, which happens during
	// bursts of file changes.
	c := make(chan notify.EventInfo, eventBufferSize)

	for _, dir := range dirs {
		dir, err := util.CleanPath(dir)
		if err != nil {
			return fmt.Errorf("invalid directory: %w", err)
		}
		err = notify.Watch(dir+"...", c, notify.Write)
		if err != nil {
			return fmt.Errorf("error watching directory recursively '%s': %w", dir, err)
		}
	}

	go func(ctx context.Context, c chan notify.EventInfo, mod chan<- string) {
		for {
			select {

			case <-ctx.Done():
				notify.Stop(c)
				log.Trace("stopping watcher")
				if err := ctx.Err(); err != nil {
					log.Trace(fmt.Errorf("error found in ctx: %w", err))
					return
				}

			case e := <-c:
				path := e.Path()

				log.Tracef("received event %s for %s", e.Event().String(), path)

				// send modification event
				mod <- path
			}
		}
	}(ctx, c, mod)

	return nil
}

var _ dirWatcher = (*defaultWatcher)(nil)

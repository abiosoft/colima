package inotify

import (
	"context"
	"fmt"
	"os"

	"github.com/abiosoft/colima/util"
	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
)

type dirWatcher interface {
	// Watch watches directories recursively for changes and sends message via c on
	// modifications to files within the watched directories.
	//
	// Watch returns a function that stops the watcher and waits for its registrations
	// to be removed. The watcher also stops when ctx is done.
	Watch(ctx context.Context, dirs []string, c chan<- modEvent) (stop func(), err error)
}

type defaultWatcher struct {
	log *logrus.Entry
}

// Watch implements dirWatcher
func (d *defaultWatcher) Watch(ctx context.Context, dirs []string, mod chan<- modEvent) (func(), error) {
	log := d.log
	c := make(chan notify.EventInfo, 1)

	for _, dir := range dirs {
		dir, err := util.CleanPath(dir)
		if err != nil {
			notify.Stop(c)
			return nil, fmt.Errorf("invalid directory: %w", err)
		}
		err = notify.Watch(dir+"...", c, notify.Write)
		if err != nil {
			notify.Stop(c)
			return nil, fmt.Errorf("error watching directory recursively '%s': %w", dir, err)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer notify.Stop(c)
		for {
			select {
			case <-ctx.Done():
				log.Trace("stopping watcher")
				return

			case e := <-c:
				path := e.Path()

				log.Tracef("received event %s for %s", e.Event().String(), path)

				stat, err := os.Stat(path)
				if err != nil {
					log.Trace(fmt.Errorf("unable to stat inotify file '%s': %w", path, err))
					continue
				}

				if stat.IsDir() {
					log.Tracef("'%s' is directory, ignoring.", path)
					continue
				}

				// send modification event
				select {
				case mod <- modEvent{path: path, FileMode: stat.Mode()}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}, nil
}

var _ dirWatcher = (*defaultWatcher)(nil)

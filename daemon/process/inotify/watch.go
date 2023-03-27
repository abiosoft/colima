package inotify

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/util"
	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
)

type dirWatcher interface {
	// Watch watches directories recursively for changes and sends message via c on
	// modifications to files within the watched directories.
	//
	// Watch blocks and only terminates when the context is done or a fatal error occurs.
	Watch(ctx context.Context, dirs []string, c chan<- modEvent) error
}

type defaultWatcher struct {
	log *logrus.Entry
}

// Watch implements dirWatcher
func (d *defaultWatcher) Watch(ctx context.Context, dirs []string, mod chan<- modEvent) error {
	log := d.log
	c := make(chan notify.EventInfo, 1)

	for _, dir := range dirs {
		dir, err := util.CleanPath(dir)
		if err != nil {
			return fmt.Errorf("invalid directory: %w", err)
		}
		err = notify.Watch(dir+"...", c, notify.Write, notify.Create, notify.Rename)
		if err != nil {
			return fmt.Errorf("error watching directory recursively '%s': %w", dir, err)
		}
	}

	go func(ctx context.Context, c chan notify.EventInfo, mod chan<- modEvent) {
		for {
			select {

			case <-ctx.Done():
				notify.Stop(c)
				if err := ctx.Err(); err != nil {
					log.Error(err)
					return
				}

			case e := <-c:
				path := e.Path()

				log.Tracef("received event %s for %s", e.Event().String(), path)
				stat, err := os.Stat(path)
				if err != nil {
					log.Trace(fmt.Errorf("unable to stat inotify file '%s': %w", path, err))

					try_path := strings.TrimSuffix(path, "~")
					if path == try_path {
						continue
					}
					stat, err = os.Stat(try_path)
					if err != nil {
						continue
					}
					log.Tracef("using updated path: %s", try_path)
					path = try_path
				}

				if stat.IsDir() {
					log.Tracef("'%s' is directory, ignoring.", path)
					continue
				}

				// send modification event
				mod <- modEvent{path: path, FileMode: stat.Mode()}
			}
		}
	}(ctx, c, mod)

	return nil
}

var _ dirWatcher = (*defaultWatcher)(nil)

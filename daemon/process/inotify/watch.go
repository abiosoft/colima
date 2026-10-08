package inotify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Watch(ctx context.Context, dirs []string, c chan<- modEvent) error
}

type defaultWatcher struct {
	log *logrus.Entry
}

// Watch implements dirWatcher
func (d *defaultWatcher) Watch(ctx context.Context, dirs []string, mod chan<- modEvent) error {
	log := d.log
	// notify drops events when the channel is full, which happens during
	// bursts of file changes.
	c := make(chan notify.EventInfo, eventBufferSize)

	for _, dir := range dirs {
		dir, err := util.CleanPath(dir)
		if err != nil {
			return fmt.Errorf("invalid directory: %w", err)
		}
		err = notify.Watch(dir+"...", c, notify.Write, notify.Create, notify.Remove, notify.Rename)
		if err != nil {
			return fmt.Errorf("error watching directory recursively '%s': %w", dir, err)
		}
	}

	go func(ctx context.Context, c chan notify.EventInfo, mod chan<- modEvent) {
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

				ev, err := syncTarget(path)
				if err != nil {
					log.Trace(err)
					continue
				}

				// send modification event
				mod <- ev
			}
		}
	}(ctx, c, mod)

	return nil
}

var _ dirWatcher = (*defaultWatcher)(nil)

// syncTarget returns the event to replay in the VM for a change to path.
//
// A file that still exists is synced directly. Otherwise, i.e. a directory
// or a removed or renamed path, the parent directory is synced so that
// watchers notice the added or removed entry.
func syncTarget(path string) (modEvent, error) {
	if strings.HasPrefix(filepath.Base(path), syncDirTempPrefix) {
		return modEvent{}, fmt.Errorf("'%s' is a sync temporary file, ignoring", path)
	}

	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return modEvent{path: path, FileMode: stat.Mode()}, nil
	}

	dir := filepath.Dir(path)
	stat, err := os.Stat(dir)
	if err != nil {
		return modEvent{}, fmt.Errorf("unable to stat inotify directory '%s': %w", dir, err)
	}
	return modEvent{path: dir, FileMode: stat.Mode()}, nil
}

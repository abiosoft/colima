package inotify

import (
	"context"
	"fmt"
	"io/fs"
	"time"
)

type modEvent struct {
	path string // filename
	fs.FileMode
}

func (m modEvent) Mode() string { return fmt.Sprintf("%o", m.FileMode) }

func (f *inotifyProcess) handleEvents(ctx context.Context, watcher dirWatcher) error {
	log := f.log
	log.Trace("begin inotify event handler")

	mod := make(chan modEvent)

	if err := watcher.Watch(ctx, f.vmVols, mod); err != nil {
		return fmt.Errorf("error starting watcher: %w", err)
	}

	var last time.Time
	for {
		select {
		case <-ctx.Done():
			close(mod)
			return ctx.Err()
		case ev := <-mod:
			now := time.Now()

			// rate limit, handle an event every 500 ms
			if now.Sub(last) < time.Millisecond*500 {
				continue
			}
			last = now

			log.Infof("refreshing mtime for %s ", ev.path)
			if err := f.guest.Run("/bin/chmod", ev.Mode(), ev.path); err != nil {
				log.Error(err)
			}
		}
	}
}

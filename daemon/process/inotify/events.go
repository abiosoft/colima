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
	vols := make(chan []string)

	if err := f.monitorContainerVolumes(ctx, vols); err != nil {
		return fmt.Errorf("error watching container volumes: %w", err)
	}

	var last time.Time
	var cancelWatch context.CancelFunc
	var currentVols []string

	volsChanged := func(vols []string) bool {
		if len(currentVols) != len(vols) {
			return true
		}
		for i := range vols {
			if vols[i] != currentVols[i] {
				return true
			}
		}
		return false
	}

	cache := map[string]struct{}{}

	for {
		select {

		// exit signal
		case <-ctx.Done():
			close(mod)
			return ctx.Err()

		// watch only container volumes
		case vols := <-vols:
			if !volsChanged(vols) {
				continue
			}
			log.Tracef("volumes changed from: %+v, to: %+v", currentVols, vols)

			currentVols = vols

			if cancel := cancelWatch; cancel != nil {
				// delay a bit to avoid zero downtime
				time.AfterFunc(time.Second*1, cancel)
			}

			ctx, cancel := context.WithCancel(ctx)
			cancelWatch = cancel

			go func(ctx context.Context, vols []string, mod chan<- modEvent) {
				if err := watcher.Watch(ctx, vols, mod); err != nil {
					log.Error(fmt.Errorf("error running watcher: %w", err))
				}
			}(ctx, vols, mod)

		// handle modification events
		case ev := <-mod:
			now := time.Now()

			// rate limit, handle at most 50 unique items every 500 ms
			if now.Sub(last) < time.Millisecond*500 {
				if _, ok := cache[ev.path]; ok {
					continue // handled, ignore
				}
				if len(cache) > 50 {
					continue
				}
			} else {
				last = now
				cache = map[string]struct{}{} // >500ms, reset unique cache
			}

			// cache current event
			cache[ev.path] = struct{}{}

			// validate that file exists
			if err := f.guest.RunQuiet("stat", ev.path); err != nil {
				log.Trace(fmt.Errorf("cannot stat '%s': %w", ev.path, err))
				continue
			}

			log.Infof("syncing inotify event for %s ", ev.path)
			if err := f.guest.RunQuiet("sudo", "/bin/chmod", ev.Mode(), ev.path); err != nil {
				log.Trace(fmt.Errorf("error syncing inotify event: %w", err))
			}
		}
	}
}

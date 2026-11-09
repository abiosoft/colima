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

const syncEventScript = `/bin/chmod "$1" "$2" && : >> "$2"`

func syncEventCmd(ev modEvent) []string {
	return []string{"sudo", "/bin/sh", "-c", syncEventScript, "sh", ev.Mode(), ev.path}
}

func (f *inotifyProcess) handleEvents(ctx context.Context, watcher dirWatcher) error {
	log := f.log
	log.Trace("begin inotify event handler")

	mod := make(chan modEvent)
	vols := make(chan []string)

	if err := f.monitorContainerVolumes(ctx, vols); err != nil {
		return fmt.Errorf("error watching container volumes: %w", err)
	}

	var last time.Time
	var stopWatch func()
	defer func() {
		if stopWatch != nil {
			stopWatch()
		}
	}()
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
			return ctx.Err()

		// watch only container volumes
		case vols := <-vols:
			if !volsChanged(vols) {
				continue
			}
			log.Tracef("volumes changed from: %+v, to: %+v", currentVols, vols)

			// Overlapping recursive watches can lose the parent watch when
			// notify removes the old child watch on macOS.
			if stopWatch != nil {
				stopWatch()
				stopWatch = nil
			}
			currentVols = nil
			if len(vols) == 0 {
				continue
			}

			stop, err := watcher.Watch(ctx, vols, mod)
			if err != nil {
				log.Error(fmt.Errorf("error running watcher: %w", err))
				continue
			}
			stopWatch = stop
			currentVols = vols

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
			if err := f.guest.RunQuiet(syncEventCmd(ev)...); err != nil {
				log.Trace(fmt.Errorf("error syncing inotify event: %w", err))
			}
		}
	}
}

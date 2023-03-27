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

	go f.monitorContainerVolumes(ctx, vols)

	var last time.Time
	var cancelWatch context.CancelFunc
	var currentVols []string

	volsChanged := func(vols []string) bool {
		if len(currentVols) != len(vols) {
			return false
		}
		for i := range vols {
			if vols[i] != currentVols[i] {
				return false
			}
		}
		return true
	}

	count := 0

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

			// rate limit, handle at most 50 events every 500 ms
			if now.Sub(last) < time.Millisecond*500 {
				count++
				if count > 50 {
					continue
				}
			} else {
				last = now
				count = 0 // >500ms, reset counter
			}

			log.Infof("refreshing mtime for %s ", ev.path)
			if err := f.guest.Run("/bin/chmod", ev.Mode(), ev.path); err != nil {
				log.Error(err)
			}
		}
	}
}

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

// syncEventsScript syncs each (mode, path) pair passed as arguments.
// Files that do not exist in the VM are skipped.
const syncEventsScript = `while [ $# -gt 1 ]; do [ -e "$2" ] && /bin/chmod "$1" "$2" && : >> "$2"; shift 2; done; true`

// syncBatchDelay is how long events are collected before they are synced.
const syncBatchDelay = 100 * time.Millisecond

// maxSyncArgsSize caps the size of the arguments of a single sync command,
// well below the 128KiB limit of a single argument on Linux, as the command
// is passed to the VM's shell as one string.
const maxSyncArgsSize = 64 * 1024

// syncEventsCmds returns the commands that sync evs, splitting them so that
// each command stays within maxSyncArgsSize.
func syncEventsCmds(evs []modEvent) [][]string {
	var cmds [][]string
	var args []string
	size := 0

	flush := func() {
		if len(args) == 0 {
			return
		}
		cmd := append([]string{"sudo", "/bin/sh", "-c", syncEventsScript, "sh"}, args...)
		cmds = append(cmds, cmd)
		args = nil
		size = 0
	}

	for _, ev := range evs {
		mode := ev.Mode()
		n := len(mode) + len(ev.path) + 2
		if size+n > maxSyncArgsSize {
			flush()
		}
		args = append(args, mode, ev.path)
		size += n
	}
	flush()

	return cmds
}

// batchEvents collects events from mod, deduplicated by path, and passes
// them to sync in batches. A batch is synced delay after its first event,
// or as soon as the previous sync returns. sync runs in the
// background so that events keep being received while it runs.
func batchEvents(ctx context.Context, mod <-chan modEvent, delay time.Duration, sync func([]modEvent)) {
	var pending []modEvent
	index := map[string]int{}
	var flush <-chan time.Time
	syncing := false
	done := make(chan struct{}, 1)

	start := func() {
		evs := pending
		pending = nil
		index = map[string]int{}
		syncing = true
		go func() {
			sync(evs)
			done <- struct{}{}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-mod:
			if !ok {
				return
			}
			if i, ok := index[ev.path]; ok {
				pending[i] = ev
				continue
			}
			index[ev.path] = len(pending)
			pending = append(pending, ev)
			if !syncing && flush == nil {
				flush = time.After(delay)
			}

		case <-flush:
			flush = nil
			start()

		case <-done:
			syncing = false
			if len(pending) > 0 {
				start()
			}
		}
	}
}

func (f *inotifyProcess) syncEvents(evs []modEvent) {
	log := f.log
	for _, ev := range evs {
		log.Tracef("syncing inotify event for %s", ev.path)
	}
	log.Infof("syncing inotify events for %d file(s)", len(evs))

	for _, cmd := range syncEventsCmds(evs) {
		if err := f.guest.RunQuiet(cmd...); err != nil {
			log.Trace(fmt.Errorf("error syncing inotify events: %w", err))
		}
	}
}

func (f *inotifyProcess) handleEvents(ctx context.Context, watcher dirWatcher) error {
	log := f.log
	log.Trace("begin inotify event handler")

	mod := make(chan modEvent)
	vols := make(chan []string)

	if err := f.monitorContainerVolumes(ctx, vols); err != nil {
		return fmt.Errorf("error watching container volumes: %w", err)
	}

	go batchEvents(ctx, mod, syncBatchDelay, f.syncEvents)

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
		}
	}
}

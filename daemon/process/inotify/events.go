package inotify

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type modEvent struct {
	path string // filename
	fs.FileMode
	info fs.FileInfo // state of the file on the host
}

func (m modEvent) Mode() string { return fmt.Sprintf("%o", m.FileMode) }

// syncEventsScript syncs the files passed as arguments after their mode.
// Files that do not exist in the VM are skipped.
//
// chmod emits IN_ATTRIB, and opening and closing a file for writing emits
// IN_CLOSE_WRITE. chmod runs once for all files, as starting a process for
// each file is what takes most of the time.
const syncEventsScript = `m=$1; shift; /bin/chmod "$m" "$@" 2>/dev/null; for f; do [ -e "$f" ] && : >> "$f"; done; true`

// syncBatchDelay is how long events are collected before they are synced.
const syncBatchDelay = 100 * time.Millisecond

// syncedTTL is how long the state of a synced file is remembered.
const syncedTTL = time.Minute

// maxSyncArgsSize caps the size of the arguments of a single sync command,
// well below the 128KiB limit of a single argument on Linux, as the command
// is passed to the VM's shell as one string.
const maxSyncArgsSize = 64 * 1024

// syncParallelism is the number of sync commands run at the same time.
const syncParallelism = 4

// syncEventsCmds returns the commands that sync evs, one per file mode,
// split so that each command stays within maxSyncArgsSize.
func syncEventsCmds(evs []modEvent) [][]string {
	byMode := map[string][]string{}
	var modes []string
	for _, ev := range evs {
		mode := ev.Mode()
		if _, ok := byMode[mode]; !ok {
			modes = append(modes, mode)
		}
		byMode[mode] = append(byMode[mode], ev.path)
	}
	sort.Strings(modes)

	var cmds [][]string
	for _, mode := range modes {
		var paths []string
		size := 0
		flush := func() {
			if len(paths) == 0 {
				return
			}
			cmd := append([]string{"sudo", "/bin/sh", "-c", syncEventsScript, "sh", mode}, paths...)
			cmds = append(cmds, cmd)
			paths = nil
			size = 0
		}
		for _, path := range byMode[mode] {
			if size+len(path)+1 > maxSyncArgsSize {
				flush()
			}
			paths = append(paths, path)
			size += len(path) + 1
		}
		flush()
	}
	return cmds
}

// batchEvents collects paths from mod, deduplicated, and passes them to
// handle in batches. A batch is handled delay after its first path, or as
// soon as the previous batch has been handled. handle runs in the
// background so that paths keep being received while it runs: notify drops
// events that are not received in time.
func batchEvents(ctx context.Context, mod <-chan string, delay time.Duration, handle func([]string)) {
	var pending []string
	seen := map[string]struct{}{}
	var flush <-chan time.Time
	handling := false
	done := make(chan struct{}, 1)

	start := func() {
		paths := pending
		pending = nil
		seen = map[string]struct{}{}
		handling = true
		go func() {
			handle(paths)
			done <- struct{}{}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case path, ok := <-mod:
			if !ok {
				return
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			pending = append(pending, path)
			if !handling && flush == nil {
				flush = time.After(delay)
			}

		case <-flush:
			flush = nil
			start()

		case <-done:
			handling = false
			if len(pending) > 0 {
				start()
			}
		}
	}
}

type runner interface {
	RunQuiet(args ...string) error
}

type syncedFile struct {
	info fs.FileInfo
	at   time.Time
}

// syncer syncs batches of changed paths to the VM. It handles one batch at
// a time.
type syncer struct {
	guest  runner
	log    *logrus.Entry
	synced map[string]syncedFile
}

func newSyncer(guest runner, log *logrus.Entry) *syncer {
	return &syncer{guest: guest, log: log, synced: map[string]syncedFile{}}
}

// unchanged reports whether a and b describe the same, unmodified file.
func unchanged(a, b fs.FileInfo) bool {
	return os.SameFile(a, b) &&
		a.Size() == b.Size() &&
		a.Mode() == b.Mode() &&
		a.ModTime().Equal(b.ModTime())
}

// events returns the events to sync for paths.
//
// Files left unchanged since they were last synced are skipped. Syncing a
// file from the VM can itself emit an event on the host, which would
// otherwise sync the file again, endlessly.
func (s *syncer) events(paths []string) []modEvent {
	now := time.Now()
	for path, f := range s.synced {
		if now.Sub(f.at) > syncedTTL {
			delete(s.synced, path)
		}
	}

	var evs []modEvent
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			s.log.Trace(fmt.Errorf("unable to stat inotify file '%s': %w", path, err))
			continue
		}
		if info.IsDir() {
			s.log.Tracef("'%s' is directory, ignoring.", path)
			continue
		}
		if f, ok := s.synced[path]; ok && unchanged(f.info, info) {
			s.log.Tracef("'%s' is unchanged since its last sync, ignoring.", path)
			continue
		}
		s.synced[path] = syncedFile{info: info, at: now}
		evs = append(evs, modEvent{path: path, FileMode: info.Mode(), info: info})
	}
	return evs
}

func (s *syncer) sync(paths []string) {
	log := s.log
	evs := s.events(paths)
	if len(evs) == 0 {
		return
	}
	for _, ev := range evs {
		log.Tracef("syncing inotify event for %s", ev.path)
	}
	log.Infof("syncing inotify events for %d file(s)", len(evs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, syncParallelism)
	for _, cmd := range syncEventsCmds(evs) {
		wg.Add(1)
		sem <- struct{}{}
		go func(cmd []string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.guest.RunQuiet(cmd...); err != nil {
				log.Trace(fmt.Errorf("error syncing inotify events: %w", err))
			}
		}(cmd)
	}
	wg.Wait()
}

func (f *inotifyProcess) handleEvents(ctx context.Context, watcher dirWatcher) error {
	log := f.log
	log.Trace("begin inotify event handler")

	mod := make(chan string)
	vols := make(chan []string)

	if err := f.monitorContainerVolumes(ctx, vols); err != nil {
		return fmt.Errorf("error watching container volumes: %w", err)
	}

	go batchEvents(ctx, mod, syncBatchDelay, newSyncer(f.guest, log).sync)

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

			go func(ctx context.Context, vols []string, mod chan<- string) {
				if err := watcher.Watch(ctx, vols, mod); err != nil {
					log.Error(fmt.Errorf("error running watcher: %w", err))
				}
			}(ctx, vols, mod)
		}
	}
}

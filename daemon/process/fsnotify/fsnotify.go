package fsnotify

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// New returns fsnotify process.
func New() process.Process {
	return &fsnotifyProcess{}
}

type fsnotifyProcess struct {
	app   app.App
	dirs  []string
	alive bool
	sync.Mutex
}

// Alive implements process.Process
func (f *fsnotifyProcess) Alive(ctx context.Context) error {
	f.Lock()
	defer f.Unlock()

	if f.alive {
		return nil
	}
	return fmt.Errorf("not running")
}

// Dependencies implements process.Process
func (*fsnotifyProcess) Dependencies() (deps []process.Dependency, root bool) {
	return nil, false
}

// Name implements process.Process
func (*fsnotifyProcess) Name() string { return "fsnotify" }

// Start implements process.Process
func (f *fsnotifyProcess) Start(ctx context.Context) error {
	app, err := app.New()
	if err != nil {
		return fmt.Errorf("error starting fsnotify: %w", err)
	}
	f.app = app

	f.waitForLima(ctx)

	c, err := lima.InstanceConfig()
	if err != nil {
		return fmt.Errorf("error retrieving config")
	}

	for _, mount := range c.MountsOrDefault() {
		p, err := mount.CleanPath()
		if err != nil {
			return fmt.Errorf("error retrieving mount path: %w", err)
		}
		f.dirs = append(f.dirs, p)
	}

	return f.watch(ctx)
}

// waitForLima waits until lima starts and sets the directory to watch.
func (f *fsnotifyProcess) waitForLima(ctx context.Context) {
	// wait for Lima to finish starting
	for {
		logrus.Trace("waiting for Lima...")

		// 5 second interval
		after := time.After(time.Second * 5)

		select {
		case <-ctx.Done():
			return
		case <-after:
			i, err := lima.Instance()
			if err == nil && i.Running() {
				return
			}
		}
	}
}

func (f *fsnotifyProcess) watch(ctx context.Context) error {
	// start watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error creating watcher: %w", err)
	}
	defer watcher.Close()

	// traverse directory and add to watch list
	for _, dir := range f.dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			// skip all hidden files/folders
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}

			if d.IsDir() {
				if err := watcher.Add(path); err != nil {
					return fmt.Errorf("error adding '%s' to watch directories: %w", path, err)
				}
			}
			return nil
		})
	}

	f.Lock()
	f.alive = true
	f.Unlock()

	// accumulate events per second and dispatch in batch
	for {
		var events []fsnotify.Event
		after := time.After(time.Second * 1)

	loop:
		for {
			select {

			case ev, ok := <-watcher.Events:
				if !ok {
					return fmt.Errorf("watcher channel closed")
				}
				logrus.Tracef("fsnotify: got event: %s, file: %s", ev.Op, ev.Name)

				// if write event
				if ev.Op&fsnotify.Write == fsnotify.Write {
					events = append(events, ev)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return fmt.Errorf("watcher channel closed")
				}
				logrus.Tracef("fsnotify: watch error: %v", err)

			case <-after:
				go f.Dispatch(events)
				break loop

			case <-ctx.Done():
				return nil
			}

		}
	}

}

func (f *fsnotifyProcess) Dispatch(events []fsnotify.Event) {
	l := len(events)

	switch {

	// nothing to do
	case l == 0:
		return

	// at most 10 events, discard the rest
	case l > 10:
		logrus.Tracef("fsnotify events more than 10 (%d), discarding the extra %d", l, l-10)
		events = events[:10]
	}

	// dispatch in parallel
	for _, ev := range events {
		logrus.Tracef("%s modified, touching...", ev.Name)
		go func(ev fsnotify.Event) {
			f.Touch(ev.Name)
		}(ev)
	}
}

func (f *fsnotifyProcess) Touch(file string) error {
	return f.app.SSH(false, "touch", file)
}

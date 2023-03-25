package inotify

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type modTime struct {
	path string // filename
	fs.FileMode
}

func (m modTime) Mode() string { return fmt.Sprintf("%o", m.FileMode) }

func (f *inotifyProcess) watchFiles(ctx context.Context) error {
	log := f.log
	log.Trace("begin inotify watcher")

	fileMap := map[string]time.Time{}
	mod := make(chan modTime)

	go func(mod chan modTime) {
		var last time.Time
		for {
			select {
			case <-ctx.Done():
				close(mod)
				return
			case ev := <-mod:
				now := time.Now()
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
	}(mod)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil {
				log.Trace(fmt.Errorf("error during watchfile: %w", err))
			}
			return err

		case <-time.After(f.interval):
			// get updated container directories
			var dirs []string

			f.RLock()
			dirs = append(dirs, f.containerVols...) // creating a copy
			f.RUnlock()

			if len(dirs) == 0 {
				continue
			}

			if err := doWatch(dirs, fileMap, mod); err != nil {
				log.Errorf("error during directory watch: %v", err)
			}
		}
	}
}

func doWatch(dirs []string, fileMap map[string]time.Time, changed chan<- modTime) error {
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			// skip hidden dirs
			if strings.HasPrefix(d.Name(), ".") && d.IsDir() {
				return fs.SkipDir
			}

			// do nothing for directories
			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			currentTime, ok := fileMap[path]
			newTime := info.ModTime()

			if ok && newTime.After(currentTime.Add(time.Millisecond*500)) {
				go func(path string) {
					logrus.Tracef("changed file mtime %v->%v: %s", currentTime, newTime, path)
					changed <- modTime{path: path, FileMode: info.Mode()}
				}(path)
			}

			fileMap[path] = newTime
			return nil
		})

		if err != nil {
			return fmt.Errorf("error during walkdir: %w", err)
		}
	}

	return nil
}

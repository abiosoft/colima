package inotify

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

func (f *inotifyProcess) watchFiles(ctx context.Context) error {
	log := f.log
	log.Trace("begin inotify watcher")

	fileMap := map[string]time.Time{}
	changed := make(chan string)

	go func(changed chan string) {
		var last time.Time
		for {
			select {
			case <-ctx.Done():
				close(changed)
				return
			case path := <-changed:
				now := time.Now()
				if now.Sub(last) < time.Second {
					continue
				}
				last = now
				log.Trace("syncing file notify for ", path)
				if err := f.guest.Run("touch", path); err != nil {
					log.Error(err)
				}
			}

		}
	}(changed)

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

			if err := doWatch(dirs, fileMap, changed); err != nil {
				log.Errorf("error during directory watch: %v", err)
			}
		}
	}
}

func doWatch(dirs []string, fileMap map[string]time.Time, changed chan<- string) error {
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

			if ok && newTime.After(currentTime) {
				go func(path string) {
					changed <- path
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

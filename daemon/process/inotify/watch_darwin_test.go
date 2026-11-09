//go:build darwin && cgo

package inotify

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestReplaceChildWatchWithParent(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "probe")
	if err := os.WriteFile(file, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}

	watcher := &defaultWatcher{log: logrus.NewEntry(logrus.New())}
	events := make(chan modEvent)
	stopChild, err := watcher.Watch(t.Context(), []string{child}, events)
	if err != nil {
		t.Fatal(err)
	}
	stopChild()

	stopParent, err := watcher.Watch(t.Context(), []string{root}, events)
	if err != nil {
		t.Fatal(err)
	}
	defer stopParent()

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-tick.C:
			if err := os.WriteFile(file, []byte("after"), 0644); err != nil {
				t.Fatal(err)
			}
		case event := <-events:
			if event.path == file {
				return
			}
		case <-timeout.C:
			t.Fatal("parent watch did not deliver the file write")
		}
	}
}

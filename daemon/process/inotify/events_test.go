package inotify

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func Test_syncEventsCmds(t *testing.T) {
	cmd := func(mode string, paths ...string) []string {
		return append([]string{"sudo", "/bin/sh", "-c", syncEventsScript, "sh", mode}, paths...)
	}

	tests := []struct {
		name string
		evs  []modEvent
		want [][]string
	}{
		{
			name: "no events",
			evs:  nil,
			want: nil,
		},
		{
			name: "one command per mode",
			evs: []modEvent{
				{path: "/Users/someone/project/main.go", FileMode: 0o644},
				{path: `/Users/someone/my "project"/run.sh`, FileMode: 0o755},
				{path: `/Users/someone/my "project"/a b.go`, FileMode: 0o644},
			},
			want: [][]string{
				cmd("644", "/Users/someone/project/main.go", `/Users/someone/my "project"/a b.go`),
				cmd("755", `/Users/someone/my "project"/run.sh`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := syncEventsCmds(tt.evs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncEventsCmds() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_syncEventsCmds_splitsLargeBatches(t *testing.T) {
	var evs []modEvent
	for i := range 2000 {
		evs = append(evs, modEvent{path: fmt.Sprintf("/Users/someone/project/%s/file%d.go", strings.Repeat("d", 60), i), FileMode: 0o644})
	}

	cmds := syncEventsCmds(evs)
	if len(cmds) < 2 {
		t.Fatalf("expected the batch to be split, got %d command(s)", len(cmds))
	}

	var paths []string
	for _, cmd := range cmds {
		size := 0
		for _, path := range cmd[6:] {
			size += len(path) + 1
		}
		if size > maxSyncArgsSize {
			t.Errorf("command arguments size %d exceeds %d", size, maxSyncArgsSize)
		}
		paths = append(paths, cmd[6:]...)
	}
	if len(paths) != len(evs) {
		t.Fatalf("expected %d paths, got %d", len(evs), len(paths))
	}
	for i, ev := range evs {
		if paths[i] != ev.path {
			t.Fatalf("path %d = %s, want %s", i, paths[i], ev.path)
		}
	}
}

func Test_batchEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mod := make(chan string)
	var mu sync.Mutex
	var batches [][]string
	release := make(chan struct{})

	go batchEvents(ctx, mod, 500*time.Millisecond, func(paths []string) {
		mu.Lock()
		batches = append(batches, paths)
		first := len(batches) == 1
		mu.Unlock()
		if first {
			<-release
		}
	})

	waitBatches := func(n int) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			mu.Lock()
			got := len(batches)
			mu.Unlock()
			if got >= n {
				return
			}
			select {
			case <-deadline:
				t.Fatalf("expected %d batch(es), got %d", n, got)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}

	for i := range 100 {
		mod <- fmt.Sprintf("/a/%d", i)
	}
	mod <- "/a/0"
	waitBatches(1)

	for i := range 50 {
		mod <- fmt.Sprintf("/b/%d", i)
	}
	mod <- "/b/0"
	close(release)
	waitBatches(2)

	mu.Lock()
	defer mu.Unlock()
	if len(batches[0]) != 100 {
		t.Errorf("first batch has %d paths, want 100", len(batches[0]))
	}
	if len(batches[1]) != 50 {
		t.Errorf("paths received during a sync: got batch of %d, want 50", len(batches[1]))
	}
}

type fakeRunner struct {
	mu   sync.Mutex
	cmds [][]string
}

func (f *fakeRunner) RunQuiet(args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, args)
	return nil
}

func Test_syncer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	script := filepath.Join(dir, "run.sh")
	sub := filepath.Join(dir, "pkg")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	guest := &fakeRunner{}
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	s := newSyncer(guest, logrus.NewEntry(logger))
	synced := func(paths ...string) [][]string {
		t.Helper()
		guest.cmds = nil
		s.sync(paths)
		sort.Slice(guest.cmds, func(i, j int) bool {
			return strings.Join(guest.cmds[i], " ") < strings.Join(guest.cmds[j], " ")
		})
		return guest.cmds
	}

	want := syncEventsCmds([]modEvent{{path: file, FileMode: 0o644}, {path: script, FileMode: 0o755}})
	if got := synced(file, script, sub, filepath.Join(dir, "missing.go")); !reflect.DeepEqual(got, want) {
		t.Errorf("first sync = %q, want %q", got, want)
	}

	if got := synced(file, script); len(got) != 0 {
		t.Errorf("unchanged files were synced again: %q", got)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(file, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want = syncEventsCmds([]modEvent{{path: file, FileMode: 0o644}})
	if got := synced(file, script); !reflect.DeepEqual(got, want) {
		t.Errorf("sync after a change = %q, want %q", got, want)
	}
}

package inotify

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_syncEventsCmds(t *testing.T) {
	prefix := []string{"sudo", "/bin/sh", "-c", syncEventsScript, "sh"}

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
			name: "events are passed as mode and path pairs",
			evs: []modEvent{
				{path: "/Users/someone/project/main.go", FileMode: 0o644},
				{path: `/Users/someone/my "project"/a b.go`, FileMode: 0o755},
			},
			want: [][]string{append(append([]string{}, prefix...),
				"644", "/Users/someone/project/main.go",
				"755", `/Users/someone/my "project"/a b.go`,
			)},
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
		args := cmd[5:]
		size := 0
		for _, arg := range args {
			size += len(arg) + 1
		}
		if size > maxSyncArgsSize {
			t.Errorf("command arguments size %d exceeds %d", size, maxSyncArgsSize)
		}
		for i := 1; i < len(args); i += 2 {
			paths = append(paths, args[i])
		}
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

	mod := make(chan modEvent)
	var mu sync.Mutex
	var batches [][]modEvent
	release := make(chan struct{})
	synced := make(chan struct{}, 10)

	go batchEvents(ctx, mod, 500*time.Millisecond, func(evs []modEvent) {
		mu.Lock()
		batches = append(batches, evs)
		first := len(batches) == 1
		mu.Unlock()
		if first {
			<-release
		}
		synced <- struct{}{}
	})

	for i := range 100 {
		mod <- modEvent{path: fmt.Sprintf("/a/%d", i), FileMode: 0o644}
	}
	mod <- modEvent{path: "/a/0", FileMode: 0o600}

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

	waitBatches(1)

	for i := range 50 {
		mod <- modEvent{path: fmt.Sprintf("/b/%d", i), FileMode: 0o644}
	}
	mod <- modEvent{path: "/b/0", FileMode: 0o600}
	close(release)

	waitBatches(2)
	<-synced
	<-synced

	mu.Lock()
	defer mu.Unlock()
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 100 {
		t.Errorf("first batch has %d events, want 100", len(batches[0]))
	}
	if got := batches[0][0]; got.path != "/a/0" || got.Mode() != "600" {
		t.Errorf("deduplicated event = (%s, %s), want latest (/a/0, 600)", got.path, got.Mode())
	}
	if len(batches[1]) != 50 {
		t.Errorf("events received during a sync: got batch of %d, want 50", len(batches[1]))
	}
	if got := batches[1][0]; got.path != "/b/0" || got.Mode() != "600" {
		t.Errorf("deduplicated event = (%s, %s), want latest (/b/0, 600)", got.path, got.Mode())
	}
}

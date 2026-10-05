package inotify

import (
	"context"
	"io"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func Test_omitChildrenDirectories(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{
			args: []string{"/", "/user", "/user/someone", "/a", "/a/ee", "/a/bb"},
			want: []string{"/"},
		},
		{
			args: []string{"/someone", "/user", "/user/someone", "/a", "/a/ee", "/a/bb", "/a"},
			want: []string{"/a", "/someone", "/user"},
		},
		{
			args: []string{"/someone", "/user/colima/projects/myworks", "/user/colima/projects", "/user/colima/projects/myworks", "/user/colima/projects", "/someone"},
			want: []string{"/someone", "/user/colima/projects"},
		},
		{
			args: []string{"/someone", "/user/colima/projects/myworks", "/user/colima/projects"},
			want: []string{"/someone", "/user/colima/projects"},
		},
		{
			args: []string{"/user/colima/projects"},
			want: []string{"/user/colima/projects"},
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := omitChildrenDirectories(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("omitChildrenDirectories() = %v, want %v", got, tt.want)
			}
		})
	}
}

// countHook is a logrus hook that counts how often a specific message is
// logged, letting the test observe the monitor goroutine without coupling to
// its internals.
type countHook struct {
	mu      sync.Mutex
	target  string
	matches int
}

func (h *countHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *countHook) Fire(e *logrus.Entry) error {
	if e.Message == h.target {
		h.mu.Lock()
		h.matches++
		h.mu.Unlock()
	}
	return nil
}

func (h *countHook) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.matches
}

// Test_monitorContainerVolumes_stopsOnContextCancel guards against a goroutine
// leak: the monitor goroutine must return when its context is cancelled.
//
// Before the fix, the <-ctx.Done() case lacked a trailing return, so the
// goroutine fell straight back into the for/select loop. Because a closed
// ctx.Done channel is always ready, the loop became a tight spin that logged
// "stop signal received" thousands of times per second and never exited.
//
// We observe that via a logrus hook and assert the line is emitted exactly
// once — only possible when the goroutine logs once and then returns.
func Test_monitorContainerVolumes_stopsOnContextCancel(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel) // the stop path logs at Trace level
	logger.SetOutput(io.Discard)

	hook := &countHook{target: "stop signal received"}
	logger.AddHook(hook)

	// guest is unused on the cancel path: fetch() only runs on the timer
	// branch (every volumesInterval), and the context is cancelled well
	// before the first tick, so the goroutine never reaches fetch().
	f := &inotifyProcess{
		runtime: "docker",
		log:     logger.WithField("context", "inotify"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	vols := make(chan []string, 1)
	if err := f.monitorContainerVolumes(ctx, vols); err != nil {
		t.Fatalf("monitorContainerVolumes: %v", err)
	}

	// trigger the stop path
	cancel()

	// Give the goroutine time to react. With the bug this window produces a
	// tight spin emitting "stop signal received" many thousands of times;
	// 100ms is ample for the fixed goroutine to log once and return.
	time.Sleep(100 * time.Millisecond)

	if got := hook.count(); got != 1 {
		t.Fatalf("expected exactly 1 \"stop signal received\" log line (goroutine must exit on ctx cancel), got %d", got)
	}
}

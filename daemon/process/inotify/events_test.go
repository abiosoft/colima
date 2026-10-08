package inotify

import (
	"io/fs"
	"reflect"
	"testing"
)

func Test_syncEventCmd(t *testing.T) {
	tests := []struct {
		name string
		ev   modEvent
		want []string
	}{
		{
			name: "regular file",
			ev:   modEvent{path: "/Users/someone/project/main.go", FileMode: 0o644},
			want: []string{"sudo", "/bin/sh", "-c", syncEventScript, "sh", "644", "/Users/someone/project/main.go"},
		},
		{
			name: "path with spaces and quotes stays a single argument",
			ev:   modEvent{path: `/Users/someone/my "project"/a b.go`, FileMode: 0o755},
			want: []string{"sudo", "/bin/sh", "-c", syncEventScript, "sh", "755", `/Users/someone/my "project"/a b.go`},
		},
		{
			name: "directory",
			ev:   modEvent{path: `/Users/someone/my "project"/pkg`, FileMode: fs.ModeDir | 0o755},
			want: []string{"sudo", "/bin/sh", "-c", syncDirEventScript, "sh", `/Users/someone/my "project"/pkg`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := syncEventCmd(tt.ev); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncEventCmd() = %q, want %q", got, tt.want)
			}
		})
	}
}

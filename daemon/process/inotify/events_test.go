package inotify

import (
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
			ev:   modEvent{path: "/Users/someone/project/main.go"},
			want: []string{"sudo", "/bin/sh", "-c", syncEventScript, "sh", "/Users/someone/project/main.go"},
		},
		{
			name: "path with spaces and quotes stays a single argument",
			ev:   modEvent{path: `/Users/someone/my "project"/a b.go`},
			want: []string{"sudo", "/bin/sh", "-c", syncEventScript, "sh", `/Users/someone/my "project"/a b.go`},
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

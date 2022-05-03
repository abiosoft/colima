package cmd

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/abiosoft/colima/config"
)

func Test_mountsFromFlag(t *testing.T) {
	tests := []struct {
		mounts []string
		want   []config.Mount
	}{
		{
			mounts: []string{
				"~:w",
			},
			want: []config.Mount{
				{Location: "~", Writable: true},
			},
		},
		{
			mounts: []string{
				"~",
			},
			want: []config.Mount{
				{Location: "~"},
			},
		},
		{
			mounts: []string{
				"/home/users", "/home/another:w", "/tmp",
			},
			want: []config.Mount{
				{Location: "/home/users"},
				{Location: "/home/another", Writable: true},
				{Location: "/tmp"},
			},
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := mountsFromFlag(tt.mounts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mountsFromFlag() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

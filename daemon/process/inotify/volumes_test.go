package inotify

import (
	"reflect"
	"strconv"
	"testing"
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
			args: []string{"/someone", "/user", "/user/someone", "/a", "/a/ee", "/a/bb"},
			want: []string{"/a", "/someone", "/user"},
		},
		{
			args: []string{"/someone", "/user/colima/projects/myworks", "/user/colima/projects"},
			want: []string{"/someone", "/user/colima/projects"},
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

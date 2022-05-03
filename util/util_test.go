package util

import (
	"strconv"
	"testing"
)

func TestAppendToPath(t *testing.T) {
	type args struct {
		path string
		dir  string
	}
	tests := []struct {
		args args
		want string
	}{
		{args: args{path: "/another/you", dir: "/user/me"}, want: "/user/me:/another/you"},
		{args: args{path: "/another/you:/user/me", dir: "/another/me"}, want: "/another/me:/another/you:/user/me"},
		{args: args{path: "", dir: "/another/me"}, want: "/another/me"},
		{args: args{path: "/another/me"}, want: "/another/me"},
		{args: args{path: "/another/you/me:/user/me/me:/user/me/you", dir: "/new"}, want: "/new:/another/you/me:/user/me/me:/user/me/you"},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := AppendToPath(tt.args.path, tt.args.dir); got != tt.want {
				t.Errorf("AppendToPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveFomPath(t *testing.T) {
	type args struct {
		path string
		dir  string
	}
	tests := []struct {
		args args
		want string
	}{
		{args: args{path: "/user/me:/another/you", dir: "/another/you"}, want: "/user/me"},
		{args: args{path: "/another/me:/another/you:/user/me", dir: "/another/me"}, want: "/another/you:/user/me"},
		{args: args{path: "/another/me:/another/you:/user/me", dir: "/another/you"}, want: "/another/me:/user/me"},
		{args: args{path: "", dir: "/another/me"}, want: ""},
		{args: args{path: "/another/me"}, want: "/another/me"},
		{args: args{path: "/another/you/me:/user/me/me:/user/me/you:/new", dir: "/new"}, want: "/another/you/me:/user/me/me:/user/me/you"},
		{args: args{path: "/another/you/me:/user/me/me:/user/me/you:/new:", dir: "/new"}, want: "/another/you/me:/user/me/me:/user/me/you"},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := RemoveFromPath(tt.args.path, tt.args.dir); got != tt.want {
				t.Errorf("RemoveFomPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

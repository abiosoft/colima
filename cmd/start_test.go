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
		{
			mounts: []string{
				"/home/users:/home/users", "/home/another:w", "/tmp:/users/tmp", "/tmp:/users/tmp:w",
			},
			want: []config.Mount{
				{Location: "/home/users", MountPoint: "/home/users"},
				{Location: "/home/another", Writable: true},
				{Location: "/tmp", MountPoint: "/users/tmp"},
				{Location: "/tmp", MountPoint: "/users/tmp", Writable: true},
			},
		},
		{
			mounts: []string{
				"none",
			},
			want: nil,
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

func Test_withRegistryMirrors(t *testing.T) {
	tests := []struct {
		name    string
		docker  map[string]any
		mirrors []string
		want    map[string]any
	}{
		{
			name:    "nil map",
			docker:  nil,
			mirrors: []string{"https://mirror.gcr.io"},
			want: map[string]any{
				"registry-mirrors": []string{"https://mirror.gcr.io"},
			},
		},
		{
			name:    "existing keys preserved",
			docker:  map[string]any{"insecure-registries": []string{"host.docker.internal:5000"}},
			mirrors: []string{"https://mirror.gcr.io"},
			want: map[string]any{
				"insecure-registries": []string{"host.docker.internal:5000"},
				"registry-mirrors":    []string{"https://mirror.gcr.io"},
			},
		},
		{
			name:    "existing mirrors replaced",
			docker:  map[string]any{"registry-mirrors": []string{"https://old.mirror"}},
			mirrors: []string{"https://new.mirror"},
			want: map[string]any{
				"registry-mirrors": []string{"https://new.mirror"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withRegistryMirrors(tt.docker, tt.mirrors); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("withRegistryMirrors() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

package inotify

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_syncTarget(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantDir bool
		wantErr bool
	}{
		{name: "existing file", path: file, want: file},
		{name: "directory syncs its parent", path: sub, want: dir, wantDir: true},
		{name: "removed file syncs its parent", path: filepath.Join(dir, "removed.go"), want: dir, wantDir: true},
		{name: "removed directory tree", path: filepath.Join(dir, "gone", "removed.go"), wantErr: true},
		{name: "sync temporary file", path: filepath.Join(dir, syncDirTempPrefix+"abc123"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := syncTarget(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("syncTarget() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.path != tt.want || got.IsDir() != tt.wantDir {
				t.Errorf("syncTarget() = (%s, dir=%v), want (%s, dir=%v)", got.path, got.IsDir(), tt.want, tt.wantDir)
			}
		})
	}

	if got, _ := syncTarget(file); got.Mode() != "640" {
		t.Errorf("syncTarget() mode = %s, want 640", got.Mode())
	}
}

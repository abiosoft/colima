package downloader

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeDownloader_Download_restartsRejectedResume(t *testing.T) {
	t.Setenv("COLIMA_CACHE_HOME", t.TempDir())

	content := []byte("0123456789abcdefghij")
	srv := newFileServer(t, content)

	dest := filepath.Join(t.TempDir(), "disk.qcow2.downloading")
	if err := os.WriteFile(dest, append(bytes.Clone(content), "extra"...), 0644); err != nil {
		t.Fatal(err)
	}

	if err := (&nativeDownloader{}).Download(Request{URL: srv.URL + "/disk.qcow2"}, dest); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

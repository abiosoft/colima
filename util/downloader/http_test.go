package downloader

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newFileServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "disk.qcow2", time.Time{}, bytes.NewReader(content))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPClient_Download_resume(t *testing.T) {
	content := []byte("0123456789abcdefghij")

	tests := []struct {
		name       string
		partial    []byte
		wantStatus int
	}{
		{name: "partial file is resumed", partial: content[:8]},
		{name: "complete file is kept", partial: content},
		{
			name:       "file larger than the remote is rejected",
			partial:    append(bytes.Clone(content), "extra"...),
			wantStatus: http.StatusRequestedRangeNotSatisfiable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFileServer(t, content)
			dest := filepath.Join(t.TempDir(), "disk.qcow2.downloading")
			if err := os.WriteFile(dest, tt.partial, 0644); err != nil {
				t.Fatal(err)
			}

			_, err := NewHTTPClient().Download(context.Background(), DownloadOptions{
				URL:            srv.URL,
				DestPath:       dest,
				ResumeFromByte: int64(len(tt.partial)),
			})

			if tt.wantStatus != 0 {
				var statusErr *HTTPStatusError
				if !errors.As(err, &statusErr) || statusErr.StatusCode != tt.wantStatus {
					t.Fatalf("Download() error = %v, want HTTP %d", err, tt.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("Download() error = %v", err)
			}
			got, err := os.ReadFile(dest)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, content) {
				t.Errorf("file content = %q, want %q", got, content)
			}
		})
	}
}

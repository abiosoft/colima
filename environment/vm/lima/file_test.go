package lima

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestParseFileInfo(t *testing.T) {
	info, err := parseFileInfo("/tmp/data.db", "42,644,1700000000,regular file\n")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "data.db" || info.Size() != 42 || info.Mode() != fs.FileMode(0o644) || !info.ModTime().Equal(time.Unix(1700000000, 0)) || info.IsDir() {
		t.Fatalf("unexpected file info: %+v", info)
	}
}

func TestParseFileInfoRejectsMalformedStat(t *testing.T) {
	for _, output := range []string{"42,644", "bad,644,1700000000,regular file", "42,999,1700000000,regular file", "42,644,bad,regular file"} {
		_, err := parseFileInfo("/tmp/data.db", output)
		if err == nil || !strings.Contains(err.Error(), "data.db") {
			t.Errorf("expected a contextual error for %q, got %v", output, err)
		}
	}
}

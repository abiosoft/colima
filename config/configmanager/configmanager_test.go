package configmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFromMounts ensures the three mount states round-trip correctly when
// loading a config file. In particular, an absent `mounts` key must fall back
// to the documented default (mount $HOME) rather than being treated the same as
// an explicit `mounts: null` (no mounts). See issue #1533.
func TestLoadFromMounts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		// wantHome is the expected result of MountsOrDefault containing the
		// default home mount.
		wantHome bool
		wantLen  int
	}{
		{
			name:     "absent key defaults to home",
			content:  "cpu: 4\nmemory: 8\n",
			wantHome: true,
			wantLen:  1,
		},
		{
			name:     "explicit empty list defaults to home",
			content:  "cpu: 4\nmounts: []\n",
			wantHome: true,
			wantLen:  1,
		},
		{
			name:     "explicit null disables mounts",
			content:  "cpu: 4\nmounts: null\n",
			wantHome: false,
			wantLen:  0,
		},
		{
			name:     "explicit mounts are preserved",
			content:  "cpu: 4\nmounts:\n  - location: /x\n",
			wantHome: false,
			wantLen:  1,
		},
	}

	dir := t.TempDir()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(dir, tt.name+".yaml")
			if err := os.WriteFile(f, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			c, err := LoadFrom(f)
			if err != nil {
				t.Fatalf("LoadFrom() error = %v", err)
			}

			mounts := c.MountsOrDefault()
			if len(mounts) != tt.wantLen {
				t.Errorf("MountsOrDefault() len = %d, want %d", len(mounts), tt.wantLen)
			}
			if tt.wantHome {
				if len(mounts) != 1 {
					t.Fatalf("expected single default home mount, got %d", len(mounts))
				}
				home, err := os.UserHomeDir()
				if err == nil && mounts[0].Location != home {
					t.Errorf("default mount location = %q, want home %q", mounts[0].Location, home)
				}
			}
		})
	}
}

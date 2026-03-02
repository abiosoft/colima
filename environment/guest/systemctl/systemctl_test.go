package systemctl

import (
	"os"
	"testing"
)

// mockGuest records args passed to Run/RunQuiet and controls whether they succeed.
type mockGuest struct {
	lastArgs []string
	err      error
}

func (m *mockGuest) Run(args ...string) error      { m.lastArgs = args; return m.err }
func (m *mockGuest) RunQuiet(args ...string) error { m.lastArgs = args; return m.err }

func TestStart(t *testing.T) {
	g := &mockGuest{}
	s := New(g)

	if err := s.Start("docker.service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertArgs(t, g.lastArgs, []string{"sudo", "systemctl", "start", "docker.service"})
}

func TestRestart(t *testing.T) {
	g := &mockGuest{}
	s := New(g)

	if err := s.Restart("containerd.service"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertArgs(t, g.lastArgs, []string{"sudo", "systemctl", "restart", "containerd.service"})
}

func TestStop(t *testing.T) {
	tests := []struct {
		name     string
		force    bool
		wantVerb string
	}{
		{name: "graceful", force: false, wantVerb: "stop"},
		{name: "force", force: true, wantVerb: "kill"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &mockGuest{}
			s := New(g)

			if err := s.Stop("docker.service", tt.force); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertArgs(t, g.lastArgs, []string{"sudo", "systemctl", tt.wantVerb, "docker.service"})
		})
	}
}

func TestActive(t *testing.T) {
	tests := []struct {
		name    string
		guestOK bool
		want    bool
	}{
		{name: "active", guestOK: true, want: true},
		{name: "inactive", guestOK: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &mockGuest{}
			if !tt.guestOK {
				g.err = os.ErrProcessDone
			}
			s := New(g)

			got := s.Active("docker.service")
			if got != tt.want {
				t.Errorf("Active() = %v, want %v", got, tt.want)
			}

			assertArgs(t, g.lastArgs, []string{"systemctl", "is-active", "docker.service"})
		})
	}
}

func TestDaemonReload(t *testing.T) {
	g := &mockGuest{}
	s := New(g)

	if err := s.DaemonReload(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertArgs(t, g.lastArgs, []string{"sudo", "systemctl", "daemon-reload"})
}

// assertArgs fails the test if got and want differ.
func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("args = %v, want %v", got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

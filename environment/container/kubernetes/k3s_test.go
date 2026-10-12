package kubernetes

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/store"
)

func TestMain(m *testing.M) {
	// config dirs are cached on first use, so point them at a temp dir before any test runs
	home, err := os.MkdirTemp("", "colima-test")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("COLIMA_HOME", home)
	_ = os.Unsetenv("LIMA_HOME")
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

type fakeGuest struct {
	environment.GuestActions
	conf map[string]string
}

func (f fakeGuest) Get(key string) string { return f.conf[key] }

func (f fakeGuest) Set(key, value string) error {
	f.conf[key] = value
	return nil
}

func Test_getPortNumber(t *testing.T) {
	config.SetProfile("k3s-port-test")
	if file := config.CurrentProfile().StoreFile(); !strings.HasPrefix(file, os.Getenv("COLIMA_HOME")) {
		t.Fatalf("store file %s is outside COLIMA_HOME", file)
	}

	tests := []struct {
		name       string
		store      store.Store
		guestPort  string
		configured int
		want       int
	}{
		{
			name:  "recreated VM on a kept disk reuses the disk port",
			store: store.Store{DiskFormatted: true, K3sListenPort: 51234},
			want:  51234,
		},
		{
			name:      "port in the VM is kept for the disk",
			store:     store.Store{DiskFormatted: true},
			guestPort: "52345",
			want:      52345,
		},
		{
			name:       "stored port is ignored without a formatted disk",
			store:      store.Store{K3sListenPort: 51234},
			configured: 6443,
			want:       6443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.Set(func(s *store.Store) { *s = tt.store }); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Reset() })

			guest := fakeGuest{conf: map[string]string{}}
			if tt.guestPort != "" {
				guest.conf[listenPortKey] = tt.guestPort
			}

			got, err := getPortNumber(guest, tt.configured)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("getPortNumber() = %d, want %d", got, tt.want)
			}
			if guest.conf[listenPortKey] != strconv.Itoa(tt.want) {
				t.Errorf("VM port = %s, want %d", guest.conf[listenPortKey], tt.want)
			}
			if s, _ := store.Load(); s.K3sListenPort != tt.want {
				t.Errorf("stored port = %d, want %d", s.K3sListenPort, tt.want)
			}
		})
	}
}

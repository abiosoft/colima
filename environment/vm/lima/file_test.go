package lima

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abiosoft/colima/cli"
)

// TestWritePropagatesMkdirFailure covers the pre-existing Write helper's
// mkdir-error branch (file.go): a failed directory creation must surface as
// an error and no write may follow.
func TestWritePropagatesMkdirFailure(t *testing.T) {
	mkdirErr := errors.New("mkdir failed")
	h := &fakeHost{runQuiet: func(args ...string) error {
		for _, a := range args {
			if a == "mkdir" {
				return mkdirErr
			}
		}
		return nil
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	err := l.Write("/etc/dnsmasq.d/01-colima.conf", []byte("x"))
	if !errors.Is(err, mkdirErr) {
		t.Fatalf("Write() err = %v; want %v", err, mkdirErr)
	}
	// no write command may follow a failed mkdir
	for _, call := range h.calls {
		if strings.Contains(strings.Join(call, " "), "cat >") {
			t.Fatalf("Write() ran write after mkdir failure; calls: %v", h.calls)
		}
	}
}

// TestStatNewFileInfo covers the pre-existing Stat/newFileInfo parsing
// (file.go): size, mode, modtime, isDir, malformed output, and stat failure.
func TestStatNewFileInfo(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "1234,644,1700000000,regular file", nil
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		info, err := l.Stat("/etc/dnsmasq.d/01-colima.conf")
		if err != nil {
			t.Fatalf("Stat() err = %v; want nil", err)
		}
		if info.Size() != 1234 {
			t.Fatalf("Size() = %d; want 1234", info.Size())
		}
		if info.Mode() != fs.FileMode(0o644) {
			t.Fatalf("Mode() = %v; want 0644", info.Mode())
		}
		if info.IsDir() {
			t.Fatal("IsDir() = true; want false for regular file")
		}
		if want := time.Unix(1700000000, 0); !info.ModTime().Equal(want) {
			t.Fatalf("ModTime() = %v; want %v", info.ModTime(), want)
		}
	})

	t.Run("directory", func(t *testing.T) {
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "0,755,1700000000,directory", nil
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		info, err := l.Stat("/etc/dnsmasq.d")
		if err != nil {
			t.Fatalf("Stat() err = %v; want nil", err)
		}
		if !info.IsDir() {
			t.Fatal("IsDir() = false; want true for directory")
		}
	})

	t.Run("malformed output", func(t *testing.T) {
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "too-few-fields", nil
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		if _, err := l.Stat("/etc/dnsmasq.d/01-colima.conf"); err == nil {
			t.Fatal("Stat() err = nil; want error for malformed stat output")
		}
	})

	t.Run("stat failure", func(t *testing.T) {
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "", errors.New("stat failed")
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		if _, err := l.Stat("/etc/dnsmasq.d/01-colima.conf"); err == nil {
			t.Fatal("Stat() err = nil; want error")
		}
	})
}

// TestFileInfoSys ensures the fileInfo Sys method stays nil.
func TestFileInfoSys(t *testing.T) {
	var f fileInfo
	if f.Sys() != nil {
		t.Fatalf("Sys() = %v; want nil", f.Sys())
	}
	if f.Name() != "" {
		t.Fatalf("Name() = %q; want empty", f.Name())
	}
}

// silence unused imports in older branches
var (
	_ = io.Reader(nil)
	_ = os.FileInfo(nil)
)

// TestReadCoversGuestRead covers the pre-existing Read helper (file.go):
// success returns content, failure propagates the wrapped error.
func TestReadCoversGuestRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "content", nil
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		got, err := l.Read("/etc/dnsmasq.d/01-colima.conf")
		if err != nil {
			t.Fatalf("Read() err = %v; want nil", err)
		}
		if got != "content" {
			t.Fatalf("Read() = %q; want content", got)
		}
	})

	t.Run("failure", func(t *testing.T) {
		readErr := errors.New("cat failed")
		h := &fakeHost{runOutput: func(args ...string) (string, error) {
			return "", readErr
		}}
		l := &limaVM{host: h, CommandChain: cli.New("vm")}
		if _, err := l.Read("/etc/dnsmasq.d/01-colima.conf"); err == nil {
			t.Fatal("Read() err = nil; want error")
		}
	})
}

//go:build linux

package inotify

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func readInotifyMasks(t *testing.T, fd int) map[int32]uint32 {
	t.Helper()
	masks := map[int32]uint32{}
	buf := make([]byte, 64*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, buf)
		if err == syscall.EAGAIN {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; offset < n; {
			ev := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			masks[ev.Wd] |= ev.Mask
			offset += syscall.SizeofInotifyEvent + int(ev.Len)
		}
	}
	return masks
}

func Test_syncEventsCmds_emitsAttribAndCloseWriteWithoutModifyingFiles(t *testing.T) {
	dir := t.TempDir()
	content := []byte("package main\n")
	paths := []string{filepath.Join(dir, "file with spaces.go"), filepath.Join(dir, `quote"d.go`)}
	missing := filepath.Join(dir, "missing.go")

	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(fd) }()

	var evs []modEvent
	before := map[string]os.FileInfo{}
	for _, path := range paths {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = stat
		evs = append(evs, modEvent{path: path, FileMode: stat.Mode()})
	}
	evs = append([]modEvent{{path: missing, FileMode: 0o644}}, evs...)

	watches := map[int32]string{}
	for _, path := range paths {
		wd, err := syscall.InotifyAddWatch(fd, path, syscall.IN_ATTRIB|syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE)
		if err != nil {
			t.Fatal(err)
		}
		watches[int32(wd)] = path
	}

	cmds := syncEventsCmds(evs)
	if len(cmds) != 1 {
		t.Fatalf("expected a single command, got %d", len(cmds))
	}
	args := cmds[0][1:]
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("sync command failed: %v: %s", err, out)
	}

	masks := readInotifyMasks(t, fd)
	for wd, path := range watches {
		mask := masks[wd]
		if mask&syscall.IN_ATTRIB == 0 {
			t.Errorf("%s: expected IN_ATTRIB, got mask %#x", path, mask)
		}
		if mask&syscall.IN_CLOSE_WRITE == 0 {
			t.Errorf("%s: expected IN_CLOSE_WRITE, got mask %#x", path, mask)
		}
		if mask&syscall.IN_MODIFY != 0 {
			t.Errorf("%s: expected no IN_MODIFY, got mask %#x", path, mask)
		}

		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !after.ModTime().Equal(before[path].ModTime()) {
			t.Errorf("%s: modification time changed: %v -> %v", path, before[path].ModTime(), after.ModTime())
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(content) {
			t.Errorf("%s: content changed: %q -> %q", path, content, got)
		}
	}

	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("missing file was created: %v", err)
	}
}

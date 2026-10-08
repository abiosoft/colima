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

func readInotifyMasks(t *testing.T, fd int) uint32 {
	t.Helper()
	var mask uint32
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
			mask |= ev.Mask
			offset += syscall.SizeofInotifyEvent + int(ev.Len)
		}
	}
	return mask
}

func Test_syncEventCmd_emitsAttribAndCloseWriteWithoutModifyingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file with spaces.go")
	content := []byte("package main\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(fd) }()
	if _, err := syscall.InotifyAddWatch(fd, path, syscall.IN_ATTRIB|syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE); err != nil {
		t.Fatal(err)
	}

	args := syncEventCmd(modEvent{path: path, FileMode: before.Mode()})[1:]
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("sync command failed: %v: %s", err, out)
	}

	mask := readInotifyMasks(t, fd)
	if mask&syscall.IN_ATTRIB == 0 {
		t.Errorf("expected IN_ATTRIB, got mask %#x", mask)
	}
	if mask&syscall.IN_CLOSE_WRITE == 0 {
		t.Errorf("expected IN_CLOSE_WRITE, got mask %#x", mask)
	}
	if mask&syscall.IN_MODIFY != 0 {
		t.Errorf("expected no IN_MODIFY, got mask %#x", mask)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("modification time changed: %v -> %v", before.ModTime(), after.ModTime())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("content changed: %q -> %q", content, got)
	}
}

func Test_syncEventCmd_directoryEmitsCreateAndDeleteWithoutLeavingFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir with spaces")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}

	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(fd) }()
	if _, err := syscall.InotifyAddWatch(fd, dir, syscall.IN_CREATE|syscall.IN_DELETE); err != nil {
		t.Fatal(err)
	}

	args := syncEventCmd(modEvent{path: dir, FileMode: stat.Mode()})[1:]
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("sync command failed: %v: %s", err, out)
	}

	mask := readInotifyMasks(t, fd)
	if mask&syscall.IN_CREATE == 0 {
		t.Errorf("expected IN_CREATE, got mask %#x", mask)
	}
	if mask&syscall.IN_DELETE == 0 {
		t.Errorf("expected IN_DELETE, got mask %#x", mask)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no leftover files, got %v", entries)
	}
}

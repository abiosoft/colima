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
			if mask&syscall.IN_CLOSE_WRITE != 0 {
				break
			}
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

func Test_syncEventCmd_emitsCloseWriteWithoutModifyingFile(t *testing.T) {
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

	args := syncEventCmd(modEvent{path: path})[1:]
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("sync command failed: %v: %s", err, out)
	}

	mask := readInotifyMasks(t, fd)
	if mask&syscall.IN_CLOSE_WRITE == 0 {
		t.Errorf("expected IN_CLOSE_WRITE, got mask %#x", mask)
	}
	if mask&syscall.IN_MODIFY != 0 {
		t.Errorf("expected no IN_MODIFY, got mask %#x", mask)
	}
	if mask&syscall.IN_ATTRIB != 0 {
		t.Errorf("expected no IN_ATTRIB, got mask %#x", mask)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("modification time changed: %v -> %v", before.ModTime(), after.ModTime())
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Errorf("mode changed: %v -> %v", before.Mode().Perm(), after.Mode().Perm())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("content changed: %q -> %q", content, got)
	}
}

func Test_syncEventCmd_preservesInContainerChmod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "init.sh")
	content := []byte("#!/bin/sh\necho init\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate in-container permission modification
	targetMode := os.FileMode(0o755)
	if err := os.Chmod(path, targetMode); err != nil {
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

	// Trigger sync event
	args := syncEventCmd(modEvent{path: path})[1:]
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("sync command failed: %v: %s", err, out)
	}

	mask := readInotifyMasks(t, fd)
	if mask&syscall.IN_CLOSE_WRITE == 0 {
		t.Errorf("expected IN_CLOSE_WRITE, got mask %#x", mask)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode().Perm() != targetMode.Perm() {
		t.Errorf("expected mode %o to be preserved, got %o", targetMode.Perm(), after.Mode().Perm())
	}
}

func Test_syncEventCmd_multiplePermissionVariants(t *testing.T) {
	modes := []os.FileMode{0o644, 0o755, 0o700, 0o600, 0o777}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "testfile")
			if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}

			args := syncEventCmd(modEvent{path: path})[1:]
			if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
				t.Fatalf("sync command failed: %v: %s", err, out)
			}

			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if after.Mode().Perm() != mode.Perm() {
				t.Errorf("expected mode %o, got %o", mode.Perm(), after.Mode().Perm())
			}
		})
	}
}

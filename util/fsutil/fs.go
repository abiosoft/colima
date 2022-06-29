package fsutil

import (
	"io/fs"
	"os"
	"testing/fstest"
)

// FS is the host filesystem implementation.
var FS FileSystem = DefaultFS{}

// MkdirAll calls FS.MakedirAll
func MkdirAll(path string, perm os.FileMode) error { return FS.MkdirAll(path, perm) }

// Open calls FS.Open
func Open(name string) (fs.File, error) { return FS.Open(name) }

// FS is abstraction for filesystem.
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	fs.FS
}

var _ FileSystem = DefaultFS{}
var _ FileSystem = fakeFS{}

// DefaultFS is the default OS implementation of FileSystem.
type DefaultFS struct{}

// Open implements FS
func (DefaultFS) Open(name string) (fs.File, error) { return os.Open(name) }

// MkdirAll implements FS
func (DefaultFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

// FakeFS is a mock FS. The following can be done in a test before usage.
//  osutil.FS = osutil.FakeFS
var FakeFS FileSystem = fakeFS{}

type fakeFS struct{}

// Open implements FileSystem
func (fakeFS) Open(name string) (fs.File, error) {
	return fstest.MapFS{name: &fstest.MapFile{
		Data: []byte("fake file - " + name),
	}}.Open(name)
}

// MkdirAll implements FileSystem
func (fakeFS) MkdirAll(path string, perm fs.FileMode) error { return nil }

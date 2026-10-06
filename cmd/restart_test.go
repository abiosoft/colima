package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
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

func Test_restartConfig(t *testing.T) {
	home := os.Getenv("COLIMA_HOME")
	config.SetProfile("restart-test")

	file := config.CurrentProfile().File()
	stateFile := config.CurrentProfile().StateFile()
	if !strings.HasPrefix(file, home) || !strings.HasPrefix(stateFile, home) {
		t.Fatalf("config files %s and %s are outside COLIMA_HOME %s", file, stateFile, home)
	}

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	t.Run("partial config gets start defaults", func(t *testing.T) {
		write(t, file, "cpu: 3\ndisk: 30\nruntime: containerd\n")

		conf, err := restartConfig()
		if err != nil {
			t.Fatalf("restartConfig() error = %v", err)
		}
		if conf.CPU != 3 || conf.Disk != 30 || conf.Runtime != "containerd" {
			t.Errorf("restartConfig() did not keep file values: cpu=%d disk=%d runtime=%q", conf.CPU, conf.Disk, conf.Runtime)
		}
		if conf.RootDisk != defaultRootDisk {
			t.Errorf("restartConfig() RootDisk = %d, want %d", conf.RootDisk, defaultRootDisk)
		}
		if conf.Binfmt == nil || !*conf.Binfmt {
			t.Errorf("restartConfig() Binfmt = %v, want true", conf.Binfmt)
		}
	})

	t.Run("missing config uses the instance state", func(t *testing.T) {
		write(t, stateFile, "cpu: 4\ndisk: 60\nruntime: containerd\n")

		conf, err := restartConfig()
		if err != nil {
			t.Fatalf("restartConfig() error = %v", err)
		}
		if conf.CPU != 4 || conf.Disk != 60 || conf.Runtime != "containerd" {
			t.Errorf("restartConfig() did not use the instance state: cpu=%d disk=%d runtime=%q", conf.CPU, conf.Disk, conf.Runtime)
		}
	})

	t.Run("unreadable config is an error", func(t *testing.T) {
		write(t, stateFile, "cpu: 4\ndisk: 60\nruntime: containerd\n")
		write(t, file, "cpu: [\n")

		if _, err := restartConfig(); err == nil {
			t.Error("restartConfig() error = nil, want an error")
		}
	})
}

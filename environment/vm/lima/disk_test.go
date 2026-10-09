package lima

import (
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
)

func TestMountRuntimeDisk_Docker(t *testing.T) {
	l := &limaVM{}
	conf := config.Config{Runtime: docker.Name}

	l.mountRuntimeDisk(conf, false)

	// 1 (diskMountScript) + 2 (PreMount) + 5 (Dirs) + 1 (PostMount) = 9
	if len(l.limaConf.Provision) != 9 {
		t.Fatalf("expected 9 provision entries, got %d", len(l.limaConf.Provision))
	}

	for i, p := range l.limaConf.Provision {
		if p.Mode != limaconfig.ProvisionModeDependency {
			t.Errorf("entry %d: expected mode dependency, got %s", i, p.Mode)
		}
	}

	// Verify PreMount scripts
	if l.limaConf.Provision[1].Script != "systemctl stop docker.service" {
		t.Errorf("unexpected script at index 1: %s", l.limaConf.Provision[1].Script)
	}
	if l.limaConf.Provision[2].Script != "systemctl stop containerd.service" {
		t.Errorf("unexpected script at index 2: %s", l.limaConf.Provision[2].Script)
	}

	// Verify bind mounts in the middle
	for i := 3; i <= 7; i++ {
		if !strings.Contains(l.limaConf.Provision[i].Script, "mount --bind") {
			t.Errorf("entry %d: expected mount --bind script, got %s", i, l.limaConf.Provision[i].Script)
		}
	}

	// Verify PostMount script
	postMountIdx := len(l.limaConf.Provision) - 1
	if l.limaConf.Provision[postMountIdx].Script != "systemctl start docker.service" {
		t.Errorf("expected post mount script 'systemctl start docker.service', got %s", l.limaConf.Provision[postMountIdx].Script)
	}
}

func TestMountRuntimeDisk_Order(t *testing.T) {
	l := &limaVM{}
	conf := config.Config{Runtime: docker.Name}

	l.mountRuntimeDisk(conf, true)

	stopDockerIdx := -1
	bindMountDockerIdx := -1
	startDockerIdx := -1

	for i, p := range l.limaConf.Provision {
		if p.Script == "systemctl stop docker.service" {
			stopDockerIdx = i
		}
		if strings.Contains(p.Script, "mount --bind") && strings.Contains(p.Script, "/var/lib/docker") {
			bindMountDockerIdx = i
		}
		if p.Script == "systemctl start docker.service" {
			startDockerIdx = i
		}
	}

	if stopDockerIdx == -1 {
		t.Fatal("missing stop docker script")
	}
	if bindMountDockerIdx == -1 {
		t.Fatal("missing bind mount /var/lib/docker script")
	}
	if startDockerIdx == -1 {
		t.Fatal("missing start docker script")
	}

	if !(stopDockerIdx < bindMountDockerIdx && bindMountDockerIdx < startDockerIdx) {
		t.Errorf("invalid sequence: stop=%d, bindMount=%d, start=%d", stopDockerIdx, bindMountDockerIdx, startDockerIdx)
	}
}

func TestMountRuntimeDisk_OtherRuntimes(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
	}{
		{
			name:    "containerd",
			runtime: containerd.Name,
		},
		{
			name:    "incus",
			runtime: incus.Name,
		},
		{
			name:    "none",
			runtime: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &limaVM{}
			conf := config.Config{Runtime: tt.runtime}
			l.mountRuntimeDisk(conf, false)

			for _, p := range l.limaConf.Provision {
				if p.Script == "systemctl start docker.service" {
					t.Errorf("runtime %s should not have docker start script", tt.runtime)
				}
			}
		})
	}
}

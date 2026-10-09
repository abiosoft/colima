package docker

import (
	"reflect"
	"testing"
)

func TestDataDisk(t *testing.T) {
	disk := DataDisk()

	if disk.FSType != "ext4" {
		t.Errorf("expected FSType ext4, got %s", disk.FSType)
	}

	expectedPreMount := []string{
		"systemctl stop docker.service",
		"systemctl stop containerd.service",
	}
	if !reflect.DeepEqual(disk.PreMount, expectedPreMount) {
		t.Errorf("expected PreMount %v, got %v", expectedPreMount, disk.PreMount)
	}

	expectedPostMount := []string{
		"systemctl start docker.service",
	}
	if !reflect.DeepEqual(disk.PostMount, expectedPostMount) {
		t.Errorf("expected PostMount %v, got %v", expectedPostMount, disk.PostMount)
	}

	if len(disk.Dirs) != len(diskDirs) {
		t.Errorf("expected %d disk dirs, got %d", len(diskDirs), len(disk.Dirs))
	}
}

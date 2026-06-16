package lima

import (
	"path/filepath"
	"testing"

	"github.com/abiosoft/colima/config"
)

func TestNewPhysicalDiskRuntimeDefaults(t *testing.T) {
	config.SetProfile("physical-test")
	defer config.SetProfile("default")

	runtime, err := newPhysicalDiskRuntime(2, config.PhysicalDisk{
		Name:     "src",
		Device:   "/dev/disk0s6",
		FSType:   "ext4",
		Writable: true,
		HostAccess: config.PhysicalDiskHostAccess{
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if runtime.RawDevice != "/dev/rdisk0s6" {
		t.Fatalf("RawDevice = %q, want /dev/rdisk0s6", runtime.RawDevice)
	}
	if runtime.Backend != "nbd" {
		t.Fatalf("Backend = %q, want nbd", runtime.Backend)
	}
	if runtime.MountPoint != "/mnt/colima/physical/src" {
		t.Fatalf("MountPoint = %q", runtime.MountPoint)
	}
	if runtime.HostAccess.Driver != "nfs" {
		t.Fatalf("HostAccess.Driver = %q, want nfs", runtime.HostAccess.Driver)
	}
	if runtime.HostAccess.MountPoint != "/Volumes/Colima/src" {
		t.Fatalf("HostAccess.MountPoint = %q", runtime.HostAccess.MountPoint)
	}
	if runtime.nbdDevice != "/dev/nbd2" {
		t.Fatalf("nbdDevice = %q, want /dev/nbd2", runtime.nbdDevice)
	}
	if runtime.nbdGuestPort != physicalDiskGuestNBDPortBase+2 {
		t.Fatalf("nbdGuestPort = %d", runtime.nbdGuestPort)
	}
	if filepath.Base(runtime.stateDir) != "src" {
		t.Fatalf("stateDir = %q", runtime.stateDir)
	}
}

func TestDiskutilMounted(t *testing.T) {
	if !diskutilMounted("Mounted:                   Yes\n") {
		t.Fatal("Mounted: Yes was not detected")
	}
	if diskutilMounted("Mounted:                   No\n") {
		t.Fatal("Mounted: No was detected as mounted")
	}
}

func TestPhysicalDiskNFSSourcePath(t *testing.T) {
	source, err := physicalDiskNFSSourcePath("/mnt/colima/physical/src")
	if err != nil {
		t.Fatal(err)
	}
	if source != "/src" {
		t.Fatalf("source = %q, want /src", source)
	}

	if _, err := physicalDiskNFSSourcePath("/mnt/src"); err == nil {
		t.Fatal("expected error for mount point outside NFS root")
	}
}

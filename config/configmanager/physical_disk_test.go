package configmanager

import (
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
)

func TestValidatePhysicalDisks(t *testing.T) {
	if !util.MacOS() {
		t.Skip("physicalDisks validation is currently macOS-specific")
	}

	tests := []struct {
		name    string
		disks   []config.PhysicalDisk
		wantErr string
	}{
		{
			name: "valid writable nfs disk",
			disks: []config.PhysicalDisk{
				{
					Name:      "src",
					Device:    "/dev/disk0s6",
					RawDevice: "/dev/rdisk0s6",
					FSType:    "ext4",
					Writable:  true,
					HostAccess: config.PhysicalDiskHostAccess{
						Enabled:    true,
						Driver:     "nfs",
						MountPoint: "/Volumes/Colima/src",
					},
				},
			},
		},
		{
			name: "whole disk rejected",
			disks: []config.PhysicalDisk{
				{Name: "src", Device: "/dev/disk0"},
			},
			wantErr: "must target a partition",
		},
		{
			name: "unsupported filesystem",
			disks: []config.PhysicalDisk{
				{Name: "src", Device: "/dev/disk0s6", FSType: "apfs"},
			},
			wantErr: "fsType",
		},
		{
			name: "duplicate host mountpoint",
			disks: []config.PhysicalDisk{
				{Name: "one", Device: "/dev/disk0s6", HostAccess: config.PhysicalDiskHostAccess{Enabled: true, MountPoint: "/Volumes/Colima/src"}},
				{Name: "two", Device: "/dev/disk0s7", HostAccess: config.PhysicalDiskHostAccess{Enabled: true, MountPoint: "/Volumes/Colima/src"}},
			},
			wantErr: "hostAccess.mountPoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePhysicalDisks(tt.disks)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("validatePhysicalDisks() error = %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("validatePhysicalDisks() expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("validatePhysicalDisks() error = %v, want containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

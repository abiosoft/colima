package configmanager

import (
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
)

func TestValidateDiskFilesystem(t *testing.T) {
	config.SetProfile("default")
	t.Cleanup(func() { config.SetProfile("default") })

	tests := []struct {
		name    string
		profile string
		fsType  string
		wantErr string
	}{
		{name: "ext4", profile: "default", fsType: "ext4"},
		{name: "xfs", profile: "default", fsType: "xfs"},
		{name: "unsupported", profile: "default", fsType: "btrfs", wantErr: "invalid diskFS"},
		{name: "xfs custom profile", profile: "custom", fsType: "xfs", wantErr: "generated filesystem label exceeds 12 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.SetProfile(tt.profile)
			err := validateDiskFilesystem(tt.fsType)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("validateDiskFilesystem() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("validateDiskFilesystem() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMounts(t *testing.T) {
	tests := []struct {
		name    string
		mounts  []config.Mount
		wantErr bool
	}{
		{name: "empty", mounts: nil, wantErr: false},
		{name: "no spaces", mounts: []config.Mount{{Location: "/Users/me/data"}}, wantErr: false},
		{name: "space in location", mounts: []config.Mount{{Location: "/Volumes/External HD"}}, wantErr: true},
		{name: "space in mountPoint", mounts: []config.Mount{{Location: "/Volumes/ext", MountPoint: "/mnt/External HD"}}, wantErr: true},
		{name: "valid then invalid", mounts: []config.Mount{{Location: "/Users/me/ok"}, {Location: "/Volumes/bad dir"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateMounts(tt.mounts); (err != nil) != tt.wantErr {
				t.Errorf("validateMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

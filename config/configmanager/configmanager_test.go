package configmanager

import (
	"testing"

	"github.com/abiosoft/colima/config"
)

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

func TestValidateDiskImage(t *testing.T) {
	tests := []struct {
		name      string
		diskImage string
		wantErr   bool
	}{
		{name: "empty", diskImage: "", wantErr: false},
		{name: "plain local path", diskImage: "/Users/me/colima.img", wantErr: false},
		{name: "relative local path", diskImage: "images/colima.img", wantErr: false},
		{name: "local path with spaces", diskImage: "/Volumes/My Disk/colima.img", wantErr: false},
		{name: "http lowercase", diskImage: "http://example.com/colima.img", wantErr: true},
		{name: "https lowercase", diskImage: "https://example.com/colima.img", wantErr: true},
		{name: "HTTP uppercase", diskImage: "HTTP://example.com/colima.img", wantErr: true},
		{name: "HTTPS uppercase", diskImage: "HTTPS://example.com/colima.img", wantErr: true},
		{name: "Https mixed case", diskImage: "Https://example.com/colima.img", wantErr: true},
		{name: "hTTp mixed case", diskImage: "hTTp://example.com/colima.img", wantErr: true},
		{name: "HtTpS mixed case", diskImage: "HtTpS://example.com/colima.img", wantErr: true},
		{name: "httplocal is a distinct scheme, not http", diskImage: "httplocal://x", wantErr: false},
		{name: "unrelated scheme not in scope", diskImage: "ftp://example.com/x", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDiskImage(tt.diskImage); (err != nil) != tt.wantErr {
				t.Errorf("validateDiskImage(%q) error = %v, wantErr %v", tt.diskImage, err, tt.wantErr)
			}
		})
	}
}

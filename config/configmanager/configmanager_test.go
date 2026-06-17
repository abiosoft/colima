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

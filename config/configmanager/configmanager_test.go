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

func TestValidateNetworkSubnet(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{name: "unset"},
		{
			name: "QEMU shared",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.0/24"},
			},
		},
		{
			name: "Krunkit shared",
			config: config.Config{
				VMType:  "krunkit",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.0/24"},
			},
		},
		{
			name: "VZ shared",
			config: config.Config{
				VMType:  "vz",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.0/24"},
			},
			wantErr: true,
		},
		{
			name: "bridged",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "bridged", Subnet: "192.168.107.0/24"},
			},
			wantErr: true,
		},
		{
			name: "invalid subnet",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "missing prefix length",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.0"},
			},
			wantErr: true,
		},
		{
			name: "IPv6 subnet",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "fd00::/64"},
			},
			wantErr: true,
		},
		{
			name: "host address",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.10/24"},
			},
			wantErr: true,
		},
		{
			name: "not enough usable addresses",
			config: config.Config{
				VMType:  "qemu",
				Network: config.Network{Mode: "shared", Subnet: "192.168.107.0/31"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateNetworkSubnet(tt.config); (err != nil) != tt.wantErr {
				t.Errorf("validateNetworkSubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

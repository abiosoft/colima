package config

import (
	"strings"
	"testing"
)

func TestParseSubnet(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    Subnet
		wantErr bool
	}{
		{
			name: "default",
			want: Subnet{
				Gateway: NetGateway,
				DHCPEnd: NetDHCPEnd,
				Netmask: NetMask,
			},
		},
		{
			name:  "custom",
			value: "192.168.107.0/24",
			want: Subnet{
				Gateway: "192.168.107.1",
				DHCPEnd: "192.168.107.254",
				Netmask: "255.255.255.0",
			},
		},
		{
			name:  "small custom subnet",
			value: "10.20.30.0/28",
			want: Subnet{
				Gateway: "10.20.30.1",
				DHCPEnd: "10.20.30.14",
				Netmask: "255.255.255.240",
			},
		},
		{
			name:  "minimum usable addresses",
			value: "10.20.30.4/30",
			want: Subnet{
				Gateway: "10.20.30.5",
				DHCPEnd: "10.20.30.6",
				Netmask: "255.255.255.252",
			},
		},
		{
			name:  "all IPv4 addresses",
			value: "0.0.0.0/0",
			want: Subnet{
				Gateway: "0.0.0.1",
				DHCPEnd: "255.255.255.254",
				Netmask: "0.0.0.0",
			},
		},
		{name: "invalid", value: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubnet(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSubnet() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseSubnet() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseSubnetInvalid(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "invalid", value: "invalid", wantErr: "invalid network subnet"},
		{name: "missing prefix length", value: "192.168.107.0", wantErr: "invalid network subnet"},
		{name: "invalid prefix length", value: "192.168.107.0/33", wantErr: "invalid network subnet"},
		{name: "IPv6", value: "fd00::/64", wantErr: "is not IPv4"},
		{name: "IPv4 mapped IPv6", value: "::ffff:192.168.107.0/120", wantErr: "is not IPv4"},
		{name: "host bits", value: "192.168.107.10/24", wantErr: `must use network address "192.168.107.0/24"`},
		{name: "31 bit prefix", value: "192.168.107.0/31", wantErr: "must contain at least two usable addresses"},
		{name: "32 bit prefix", value: "192.168.107.0/32", wantErr: "must contain at least two usable addresses"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubnet(tt.value)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseSubnet() error = %v, want containing %q", err, tt.wantErr)
			}
			if got != (Subnet{}) {
				t.Errorf("ParseSubnet() = %#v on error, want zero subnet", got)
			}
		})
	}
}

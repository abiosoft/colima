package lima

import (
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/util"
)

func Test_usesVmnet(t *testing.T) {
	hostArch := environment.HostArch()
	foreignArch := environment.X8664
	if hostArch == environment.X8664 {
		foreignArch = environment.AARCH64
	}

	tests := []struct {
		name string
		conf config.Config
		want bool
	}{
		{
			name: "qemu",
			conf: config.Config{VMType: limaconfig.QEMU, Arch: string(hostArch)},
			want: true,
		},
		{
			name: "vz bridged",
			conf: config.Config{VMType: limaconfig.VZ, Arch: string(hostArch), Network: config.Network{Mode: "bridged"}},
			want: true,
		},
		{
			name: "vz foreign arch falls back to qemu",
			conf: config.Config{VMType: limaconfig.VZ, Arch: string(foreignArch)},
			want: true,
		},
		{
			// VZNAT is used instead, unless vz is unavailable and qemu is used
			name: "vz shared",
			conf: config.Config{VMType: limaconfig.VZ, Arch: string(hostArch)},
			want: !util.MacOS13OrNewer(),
		},
		{
			name: "krunkit foreign arch falls back to qemu",
			conf: config.Config{VMType: limaconfig.Krunkit, Arch: string(foreignArch)},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usesVmnet(tt.conf); got != tt.want {
				t.Errorf("usesVmnet() = %v, want %v", got, tt.want)
			}
		})
	}
}

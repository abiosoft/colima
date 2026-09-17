package lima

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/fsutil"
)

func Test_checkOverlappingMounts(t *testing.T) {
	type args struct {
		mounts []string
	}
	tests := []struct {
		args    args
		wantErr bool
	}{
		{args: args{mounts: []string{"/User", "/User/something"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one", "/User/two"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one", "/User/one_other"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one_other", "/User/one"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one", "/User/one/other"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one/", "/User/one"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one/", "/User/two", "User/one"}}, wantErr: true},
		{args: args{mounts: []string{"/home/a/b/c", "/home/b/c/a", "/home/c/a/b"}}, wantErr: false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			mounts := func(mounts []string) (mnts []config.Mount) {
				for _, m := range mounts {
					mnts = append(mnts, config.Mount{Location: m})
				}
				return
			}(tt.args.mounts)
			if err := checkOverlappingMounts(mounts); (err != nil) != tt.wantErr {
				t.Errorf("checkOverlappingMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_config_Mounts(t *testing.T) {
	fsutil.FS = fsutil.FakeFS
	tests := []struct {
		mounts    []string
		isDefault bool
	}{
		{mounts: []string{"/User/user", "/tmp/another"}},
		{mounts: []string{"/User/another", "/User/something", "/User/else"}},
		{mounts: []string{}, isDefault: true},
		{mounts: nil},
		{mounts: []string{util.HomeDir()}},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			mounts := func(mounts []string) (mnts []config.Mount) {
				if mounts != nil {
					mnts = []config.Mount{}
				}

				for _, m := range mounts {
					mnts = append(mnts, config.Mount{Location: m})
				}
				return
			}(tt.mounts)
			conf, err := newConf(context.Background(), config.Config{Mounts: mounts})
			if err != nil {
				t.Error(err)
				return
			}

			expectedLocations := tt.mounts
			if tt.isDefault {
				expectedLocations = []string{"~"}
			}

			sameMounts := func(expectedLocations []string, mounts []limaconfig.Mount) bool {
				sanitize := func(s string) string { return strings.TrimSuffix(s, "/") + "/" }
				for i, m := range mounts {
					if sanitize(m.Location) != sanitize(expectedLocations[i]) {
						return false
					}
				}
				return true
			}(expectedLocations, conf.Mounts)
			if !sameMounts {
				foundLocations := func() (locations []string) {
					for _, m := range conf.Mounts {
						locations = append(locations, m.Location)
					}
					return
				}()
				t.Errorf("got: %+v, want: %v", foundLocations, expectedLocations)
			}
		})
	}
}

func Test_config_MountType(t *testing.T) {
	fsutil.FS = fsutil.FakeFS
	tests := []struct {
		vmType    string
		mountType string
		want      limaconfig.MountType
	}{
		// vz and krunkit only support virtiofs and reverse-sshfs
		{vmType: limaconfig.VZ, mountType: "virtiofs", want: limaconfig.VIRTIOFS},
		{vmType: limaconfig.Krunkit, mountType: "virtiofs", want: limaconfig.VIRTIOFS},
		// qemu does not support virtiofs, 9p is used
		{vmType: limaconfig.QEMU, mountType: "virtiofs", want: limaconfig.NINEP},
		{vmType: limaconfig.QEMU, mountType: "9p", want: limaconfig.NINEP},
		// sshfs is supported by all vm types
		{vmType: limaconfig.QEMU, mountType: "sshfs", want: limaconfig.REVSSHFS},
		{vmType: limaconfig.VZ, mountType: "sshfs", want: limaconfig.REVSSHFS},
		{vmType: limaconfig.Krunkit, mountType: "sshfs", want: limaconfig.REVSSHFS},
	}
	for _, tt := range tests {
		t.Run(tt.vmType+"_"+tt.mountType, func(t *testing.T) {
			conf := config.Config{
				VMType:    tt.vmType,
				MountType: tt.mountType,
				Arch:      string(environment.HostArch()),
			}
			l, err := newConf(context.Background(), conf)
			if err != nil {
				t.Fatal(err)
			}
			// vm types not available on the host fall back to qemu
			if l.VMType != limaconfig.VMType(tt.vmType) {
				t.Skipf("%s is not available on this host, using %s", tt.vmType, l.VMType)
			}
			if l.MountType != tt.want {
				t.Errorf("mount type = %v, want %v", l.MountType, tt.want)
			}
		})
	}
}

func Test_ingressDisabled(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"--flag=f", "--another", "flag"}, want: false},
		{args: []string{"--disable=traefik", "--version=3"}, want: true},
		{args: []string{}, want: false},
		{args: []string{"--disable", "traefik", "--one=two"}, want: true},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			if got := ingressDisabled(tt.args); got != tt.want {
				t.Errorf("ingressDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_limaVMType(t *testing.T) {
	hostArch := environment.HostArch()
	foreignArch := environment.X8664
	if hostArch == environment.X8664 {
		foreignArch = environment.AARCH64
	}

	vz := limaconfig.QEMU
	if util.MacOS13OrNewer() {
		vz = limaconfig.VZ
	}
	krunkit := limaconfig.QEMU
	if util.MacOS13OrNewerOnArm() {
		krunkit = limaconfig.Krunkit
	}

	tests := []struct {
		vmType string
		arch   environment.Arch
		want   limaconfig.VMType
	}{
		{vmType: limaconfig.QEMU, arch: hostArch, want: limaconfig.QEMU},
		{vmType: limaconfig.VZ, arch: hostArch, want: vz},
		{vmType: limaconfig.Krunkit, arch: hostArch, want: krunkit},
		// vz and krunkit cannot emulate a foreign architecture, qemu is used instead
		{vmType: limaconfig.QEMU, arch: foreignArch, want: limaconfig.QEMU},
		{vmType: limaconfig.VZ, arch: foreignArch, want: limaconfig.QEMU},
		{vmType: limaconfig.Krunkit, arch: foreignArch, want: limaconfig.QEMU},
	}
	for _, tt := range tests {
		t.Run(tt.vmType+"_"+string(tt.arch), func(t *testing.T) {
			conf := config.Config{VMType: tt.vmType, Arch: string(tt.arch)}
			if got := limaVMType(conf); got != tt.want {
				t.Errorf("limaVMType() = %v, want %v", got, tt.want)
			}
		})
	}
}

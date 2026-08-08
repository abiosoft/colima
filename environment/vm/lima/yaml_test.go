package lima

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
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

// TestNewConfResolverCompatibility characterizes the DNS/hostResolver contract
// preserved by the dnsmasq DNS path: empty DNS keeps the lima hostResolver
// enabled, explicit resolvers disable it and are preserved in exact order, and
// DNSHosts are preserved with the host.docker.internal default entry.
func TestNewConfResolverCompatibility(t *testing.T) {
	fsutil.FS = fsutil.FakeFS

	t.Run("empty DNS keeps hostResolver enabled", func(t *testing.T) {
		conf, err := newConf(context.Background(), config.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if len(conf.DNS) != 0 {
			t.Fatalf("conf.DNS = %v; want empty", conf.DNS)
		}
		if !conf.HostResolver.Enabled {
			t.Fatal("conf.HostResolver.Enabled = false; want true with empty DNS")
		}
		if got := conf.HostResolver.Hosts["host.docker.internal"]; got != "host.lima.internal" {
			t.Fatalf("host.docker.internal = %q; want %q", got, "host.lima.internal")
		}
	})

	t.Run("explicit resolvers preserved in order", func(t *testing.T) {
		resolvers := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}
		conf, err := newConf(context.Background(), config.Config{Network: config.Network{DNSResolvers: resolvers}})
		if err != nil {
			t.Fatal(err)
		}
		if len(conf.DNS) != 2 {
			t.Fatalf("conf.DNS = %v; want two resolvers", conf.DNS)
		}
		if got := conf.DNS[0].String(); got != "1.1.1.1" {
			t.Fatalf("conf.DNS[0] = %q; want %q", got, "1.1.1.1")
		}
		if got := conf.DNS[1].String(); got != "8.8.8.8" {
			t.Fatalf("conf.DNS[1] = %q; want %q", got, "8.8.8.8")
		}
		if conf.HostResolver.Enabled {
			t.Fatal("conf.HostResolver.Enabled = true; want false with explicit resolvers")
		}
	})

	t.Run("DNSHosts preserved with default entry", func(t *testing.T) {
		hosts := map[string]string{"custom.internal": "10.0.0.5"}
		conf, err := newConf(context.Background(), config.Config{Network: config.Network{DNSHosts: hosts}})
		if err != nil {
			t.Fatal(err)
		}
		if got := conf.HostResolver.Hosts["custom.internal"]; got != "10.0.0.5" {
			t.Fatalf("custom.internal = %q; want %q", got, "10.0.0.5")
		}
		if got := conf.HostResolver.Hosts["host.docker.internal"]; got != "host.lima.internal" {
			t.Fatalf("host.docker.internal = %q; want %q", got, "host.lima.internal")
		}
	})
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

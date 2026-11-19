package lima

import (
	"reflect"
	"testing"

	"github.com/abiosoft/colima/config"
)

func Test_dnsmasqHosts(t *testing.T) {
	const gateway = "192.168.5.2"
	const internalIP = "192.168.5.15"

	tests := []struct {
		name     string
		hostname string
		dnsHosts map[string]string
		want     map[string]string
	}{
		{
			name: "defaults",
			want: map[string]string{
				"host.docker.internal": gateway,
				"host.lima.internal":   gateway,
				"colima.internal":      internalIP,
			},
		},
		{
			name:     "hostname",
			hostname: "colima",
			want: map[string]string{
				"host.docker.internal": gateway,
				"host.lima.internal":   gateway,
				"colima.internal":      internalIP,
				"colima":               localhostAddr,
			},
		},
		{
			name: "user hosts",
			dnsHosts: map[string]string{
				"foo.test":             "10.1.2.3",
				"bar.test":             "fd00::1",
				"host.docker.internal": "10.9.9.9",
				"alias.test":           "host.lima.internal",
			},
			want: map[string]string{
				"host.docker.internal": "10.9.9.9",
				"host.lima.internal":   gateway,
				"colima.internal":      internalIP,
				"foo.test":             "10.1.2.3",
				"bar.test":             "fd00::1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var conf config.Config
			conf.Hostname = tt.hostname
			conf.Network.DNSHosts = tt.dnsHosts
			if got := dnsmasqHosts(conf, gateway, internalIP); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dnsmasqHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

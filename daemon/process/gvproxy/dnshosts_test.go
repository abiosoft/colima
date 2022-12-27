package gvproxy

import (
	"fmt"
	"net"
	"testing"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
)

func Test_hostsMapIP(t *testing.T) {
	hosts := hostMap{}
	hosts["sample"] = "1.1.1.1"
	hosts["another.sample"] = "1.2.2.1"
	hosts["google.com"] = "8.8.8.8"
	hosts["google.ae"] = "google.com"
	hosts["google.ie"] = "google.ae"

	tests := []struct {
		host string
		want net.IP
	}{
		{host: "sample", want: net.ParseIP("1.1.1.1")},
		{host: "another.sample", want: net.ParseIP("1.2.2.1")},
		{host: "google.com", want: net.ParseIP("8.8.8.8")},
		{host: "google.ae", want: net.ParseIP("8.8.8.8")},
		{host: "google.ie", want: net.ParseIP("8.8.8.8")},
		{host: "google.sample", want: nil},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			got := hosts.hostIP(tt.host)
			if !got.Equal(tt.want) {
				t.Errorf("hostsMapIP() = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func Test_zoneHost(t *testing.T) {
	type val struct {
		name       string
		recordName string
	}
	tests := []struct {
		host zoneHost
		want val
	}{
		{}, // test for empty value as well
		{host: "sample", want: val{name: "sample"}},
		{host: "another.sample", want: val{name: "sample.", recordName: "another"}},
		{host: "another.sample.com", want: val{name: "com.", recordName: "another.sample"}},
		{host: "a.c", want: val{name: "c.", recordName: "a"}},
		{host: "a.b.c.d", want: val{name: "d.", recordName: "a.b.c"}},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			got := val{
				name:       tt.host.name(),
				recordName: tt.host.recordName(),
			}
			if got != tt.want {
				t.Errorf("host = %+v, want %+v", got, tt.want)
				return
			}
		})
	}
}

func Test_extractZones(t *testing.T) {
	equalZones := func(za, zb []types.Zone) bool {
		find := func(list []types.Zone, name string) (types.Zone, bool) {
			for _, z := range list {
				if z.Name == name {
					return z, true
				}
			}
			return types.Zone{}, false
		}
		equal := func(a, b types.Zone) bool {
			if a.Name != b.Name {
				return false
			}
			if !a.DefaultIP.Equal(b.DefaultIP) {
				return false
			}
			for i := range a.Records {
				a, b := a.Records[i], b.Records[i]
				if !a.IP.Equal(b.IP) {
					return false
				}
				if a.Name != b.Name {
					return false
				}
			}

			return true
		}

		for _, a := range za {
			b, ok := find(zb, a.Name)
			if !ok {
				return false
			}
			if !equal(a, b) {
				return false
			}
		}
		return true
	}

	hosts := hostMap{
		"google.com":           "8.8.4.4",
		"local.google.com":     "8.8.8.8",
		"google.ae":            "google.com",
		"localhost":            "127.0.0.1",
		"host.lima.internal":   "192.168.5.2",
		"host.docker.internal": "host.lima.internal",
	}

	tests := []struct {
		wantZones []types.Zone
	}{
		{
			wantZones: []types.Zone{
				{
					Name: "com.",
					Records: []types.Record{
						{Name: "google", IP: net.ParseIP("8.8.4.4")},
						{Name: "local.google", IP: net.ParseIP("8.8.8.8")},
						{Name: "local.google", IP: net.ParseIP("8.8.8.8")},
					},
				},
				{
					Name: "ae.",
					Records: []types.Record{
						{Name: "google", IP: net.ParseIP("8.8.4.4")},
					},
				},
				{
					Name:      "localhost",
					DefaultIP: net.ParseIP("127.0.0.1"),
				},
				{
					Name: "internal.",
					Records: []types.Record{
						{Name: "host.lima", IP: net.ParseIP("192.168.5.2")},
						{Name: "host.docker", IP: net.ParseIP("192.168.5.2")},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if gotZones := extractZones(hosts); !equalZones(gotZones, tt.wantZones) {
				t.Errorf("extractZones() = %+v, want %+v", gotZones, tt.wantZones)
			}
		})
	}
}

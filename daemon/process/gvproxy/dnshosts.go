package gvproxy

import (
	"net"
	"strings"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
)

func extractZones(hosts hostMap) (zones []types.Zone) {
	list := make(map[string]types.Zone)

	for host := range hosts {
		h := zoneHost(host)

		zone := types.Zone{Name: h.name()}
		if existingZone, ok := list[h.name()]; ok {
			zone = existingZone
		}

		if h.recordName() == "" {
			if zone.DefaultIP == nil {
				zone.DefaultIP = hosts.hostIP(host)
			}
		} else {
			zone.Records = append(zone.Records, types.Record{
				Name: h.recordName(),
				IP:   hosts.hostIP(host),
			})
		}

		list[h.name()] = zone
	}

	for _, zone := range list {
		zones = append(zones, zone)
	}
	return
}

type hostMap map[string]string

func (z hostMap) hostIP(host string) net.IP {
	for {
		// check if host entry exists
		h, ok := z[host]
		if !ok || h == "" {
			return nil
		}

		// if it's a valid ip, return
		if ip := net.ParseIP(h); ip != nil {
			return ip
		}

		// otherwise, a string i.e. another host
		// loop through the process again.
		host = h
	}
}

type zoneHost string

func (z zoneHost) name() string {
	i := z.dotIndex()
	if i < 0 {
		return string(z)
	}
	return string(z)[i+1:] + "."
}

func (z zoneHost) recordName() string {
	i := z.dotIndex()
	if i < 0 {
		return ""
	}
	return string(z)[:i]
}

func (z zoneHost) dotIndex() int {
	return strings.LastIndex(string(z), ".")
}

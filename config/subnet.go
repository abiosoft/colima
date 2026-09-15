package config

import (
	"fmt"
	"net"
	"net/netip"
)

const (
	NetGateway = "192.168.106.1"
	NetDHCPEnd = "192.168.106.254"
	NetMask    = "255.255.255.0"
)

// Subnet contains the gateway, DHCP end address, and netmask for shared networking.
type Subnet struct {
	Gateway string
	DHCPEnd string
	Netmask string
}

// ParseSubnet parses and validates an IPv4 network CIDR with at least two usable
// addresses, then derives its shared networking settings. An empty value uses
// the default subnet. The CIDR address must have no host bits set.
func ParseSubnet(value string) (Subnet, error) {
	if value == "" {
		return Subnet{
			Gateway: NetGateway,
			DHCPEnd: NetDHCPEnd,
			Netmask: NetMask,
		}, nil
	}

	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return Subnet{}, fmt.Errorf("invalid network subnet %q: %w", value, err)
	}
	if !prefix.Addr().Is4() {
		return Subnet{}, fmt.Errorf("network subnet %q is not IPv4", value)
	}
	if prefix.Bits() > 30 {
		return Subnet{}, fmt.Errorf("network subnet %q must contain at least two usable addresses", value)
	}
	if prefix != prefix.Masked() {
		return Subnet{}, fmt.Errorf("network subnet %q must use network address %q", value, prefix.Masked())
	}

	mask := net.CIDRMask(prefix.Bits(), 32)
	// Set all host bits to get the subnet's broadcast address.
	broadcast := prefix.Addr().As4()
	for i := range broadcast {
		broadcast[i] |= ^mask[i]
	}

	return Subnet{
		Gateway: prefix.Addr().Next().String(),
		DHCPEnd: netip.AddrFrom4(broadcast).Prev().String(),
		Netmask: net.IP(mask).String(),
	}, nil
}

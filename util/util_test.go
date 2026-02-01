package util

import (
	"net"
	"testing"
)

func TestLoopbackIPAddresses(t *testing.T) {
	addresses := LoopbackIPAddresses()
	
	// Should return at least one address (127.0.0.1)
	if len(addresses) == 0 {
		t.Fatal("Expected at least one loopback address")
	}
	
	// Check if 127.0.0.1 is included
	has127_0_0_1 := false
	for _, addr := range addresses {
		if addr.String() == "127.0.0.1" {
			has127_0_0_1 = true
			break
		}
	}
	
	if !has127_0_0_1 {
		t.Error("Expected 127.0.0.1 to be included in loopback addresses")
	}
	
	// All addresses should be loopback (127.x.x.x)
	for _, addr := range addresses {
		if addr[0] != 127 {
			t.Errorf("Address %s is not a loopback address", addr.String())
		}
	}
}

func TestHostIPAddresses(t *testing.T) {
	addresses := HostIPAddresses()
	
	// Should not include 127.0.0.1
	for _, addr := range addresses {
		if addr.String() == "127.0.0.1" {
			t.Error("Expected 127.0.0.1 to be excluded from host addresses")
		}
	}
	
	// All addresses should be valid IPv4
	for _, addr := range addresses {
		if addr.To4() == nil {
			t.Errorf("Address %s is not a valid IPv4 address", addr.String())
		}
	}
}

func TestLoopbackAddressDetection(t *testing.T) {
	// Test that we can detect different loopback addresses
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.5", true},
		{"127.0.1.1", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
	}
	
	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Errorf("Invalid IP address: %s", tc.ip)
			continue
		}
		
		// Convert to IPv4 for loopback detection
		ip4 := ip.To4()
		isLoopback := ip4 != nil && ip4[0] == 127
		if isLoopback != tc.expected {
			t.Errorf("Expected %s to be loopback: %v, got: %v", tc.ip, tc.expected, isLoopback)
		}
	}
}

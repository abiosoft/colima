package native

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// HostResources returns the host's CPU count, total memory (bytes), and root disk size (bytes).
// This replaces limautil.Instance() which returns VM resource info.
func HostResources() (cpu int, memory int64, disk int64) {
	cpu = runtime.NumCPU()

	// Parse /proc/meminfo for total memory
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						memory = kb * 1024 // convert kB to bytes
					}
				}
				break
			}
		}
	}

	// Get root filesystem size
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		disk = int64(stat.Blocks) * int64(stat.Bsize)
	}

	return
}

// HostIPAddress returns the primary non-loopback IP address of the host.
// Falls back to "127.0.0.1" if no suitable address is found.
func HostIPAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}

// HostPrimaryInterface returns the name of the primary network interface.
func HostPrimaryInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "eth0"
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}

		// Return first interface with a non-loopback address
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return iface.Name
			}
		}
	}

	return "eth0"
}

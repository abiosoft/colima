package qemu

import "strings"

// Qemu binaries
const (
	BinAARCH64 = "qemu-system-aarch64"
	BinX8664   = "qemu-system-x86_64"
)

// Binaries is a list of Qemu binaries
var Binaries = []string{
	BinAARCH64,
	BinX8664,
}

// InstallDir is a typical Unix installation directory that contains `bin` and `share`.

// BinsEnvVar returns the environment variables for the Qemu binaries.
//
//	QEMU_SYSTEM_X86_64=/path/to/x86-bin
//	QEMU_SYSTEM_AARCH64=/path/to/aarch64-bin
// args = append(args,
// 	"-netdev", "stream,id=vlan,addr.type=unix,addr.path="+gvproxyInfo.Socket.File(),
// 	"-device", "virtio-net-pci,netdev=vlan,mac="+gvproxyInfo.MacAddress,
// )

func BinsEnvVar(args []string) []string {
	return []string{
		"QEMU_SYSTEM_X86_64=" + BinX8664 + " " + strings.Join(args, " "),
		"QEMU_SYSTEM_AARCH64=" + BinAARCH64 + " " + strings.Join(args, " "),
	}
}

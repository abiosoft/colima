package limaconfig

import (
	"net"

	"github.com/abiosoft/colima/environment"
)

type Arch = environment.Arch

// Config is lima config. Code copied from lima and modified.
type Config struct {
	VMType               VMType            `yaml:"vmType,omitempty" json:"vmType,omitempty"`
	Arch                 Arch              `yaml:"arch,omitempty"`
	Images               []File            `yaml:"images"`
	CPUs                 *int              `yaml:"cpus,omitempty"`
	Memory               string            `yaml:"memory,omitempty"`
	Disk                 string            `yaml:"disk,omitempty"`
	AdditionalDisks      []Disk            `yaml:"additionalDisks,omitempty" json:"additionalDisks,omitempty"`
	Mounts               []Mount           `yaml:"mounts,omitempty"`
	MountType            MountType         `yaml:"mountType,omitempty" json:"mountType,omitempty"`
	SSH                  SSH               `yaml:"ssh"`
	Containerd           Containerd        `yaml:"containerd"`
	Env                  map[string]string `yaml:"env,omitempty"`
	DNS                  []net.IP          `yaml:"dns"`
	Firmware             Firmware          `yaml:"firmware"`
	HostResolver         HostResolver      `yaml:"hostResolver"`
	PortForwards         []PortForward     `yaml:"portForwards,omitempty"`
	Networks             []Network         `yaml:"networks,omitempty"`
	Provision            []Provision       `yaml:"provision,omitempty" json:"provision,omitempty"`
	CPUType              map[Arch]string   `yaml:"cpuType,omitempty" json:"cpuType,omitempty"`
	Rosetta              Rosetta           `yaml:"rosetta,omitempty" json:"rosetta,omitempty"`
	NestedVirtualization bool              `yaml:"nestedVirtualization,omitempty" json:"nestedVirtualization,omitempty"`
}

type File struct {
	Location string `yaml:"location"` // REQUIRED
	Arch     Arch   `yaml:"arch,omitempty"`
	Digest   string `yaml:"digest,omitempty"`
}

type Mount struct {
	Location   string `yaml:"location"` // REQUIRED
	MountPoint string `yaml:"mountPoint,omitempty"`
	Writable   bool   `yaml:"writable"`
	NineP      NineP  `yaml:"9p,omitempty" json:"9p,omitempty"`
}

type Disk struct {
	Name   string   `yaml:"name" json:"name"` // REQUIRED
	Format *bool    `yaml:"format,omitempty" json:"format,omitempty"`
	FSType *string  `yaml:"fsType,omitempty" json:"fsType,omitempty"`
	FSArgs []string `yaml:"fsArgs,omitempty" json:"fsArgs,omitempty"`
}

type SSH struct {
	LocalPort         int  `yaml:"localPort,omitempty"`
	LoadDotSSHPubKeys bool `yaml:"loadDotSSHPubKeys"`
	ForwardAgent      bool `yaml:"forwardAgent"` // default: false
}

type Containerd struct {
	System bool `yaml:"system"` // default: false
	User   bool `yaml:"user"`   // default: true
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS bool `yaml:"legacyBIOS"`
}

type (
	Proto     = string
	MountType = string
	VMType    = string
)

const (
	TCP Proto = "tcp"

	REVSSHFS MountType = "reverse-sshfs"
	NINEP    MountType = "9p"
	VIRTIOFS MountType = "virtiofs"

	QEMU VMType = "qemu"
	VZ   VMType = "vz"
)

type PortForward struct {
	GuestIPMustBeZero bool   `yaml:"guestIPMustBeZero,omitempty" json:"guestIPMustBeZero,omitempty"`
	GuestIP           net.IP `yaml:"guestIP,omitempty" json:"guestIP,omitempty"`
	GuestPort         int    `yaml:"guestPort,omitempty" json:"guestPort,omitempty"`
	GuestPortRange    [2]int `yaml:"guestPortRange,omitempty" json:"guestPortRange,omitempty"`
	GuestSocket       string `yaml:"guestSocket,omitempty" json:"guestSocket,omitempty"`
	HostIP            net.IP `yaml:"hostIP,omitempty" json:"hostIP,omitempty"`
	HostPort          int    `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	HostPortRange     [2]int `yaml:"hostPortRange,omitempty" json:"hostPortRange,omitempty"`
	HostSocket        string `yaml:"hostSocket,omitempty" json:"hostSocket,omitempty"`
	Proto             Proto  `yaml:"proto,omitempty" json:"proto,omitempty"`
	Ignore            bool   `yaml:"ignore,omitempty" json:"ignore,omitempty"`
}

type HostResolver struct {
	Enabled bool              `yaml:"enabled" json:"enabled"`
	IPv6    bool              `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
	Hosts   map[string]string `yaml:"hosts,omitempty" json:"hosts,omitempty"`
}

type Network struct {
	// `Lima`, `Socket`, and `VNL` are mutually exclusive; exactly one is required
	Lima string `yaml:"lima,omitempty" json:"lima,omitempty"`
	// Socket is a QEMU-compatible socket
	Socket string `yaml:"socket,omitempty" json:"socket,omitempty"`
	// VZNAT uses VZNATNetworkDeviceAttachment. Needs VZ. No root privilege is required.
	VZNAT bool `yaml:"vzNAT,omitempty" json:"vzNAT,omitempty"`

	// VNLDeprecated is a Virtual Network Locator (https://github.com/rd235/vdeplug4/commit/089984200f447abb0e825eb45548b781ba1ebccd).
	// On macOS, only VDE2-compatible form (optionally with vde:// prefix) is supported.
	// VNLDeprecated is deprecated. Use Socket.
	VNLDeprecated        string `yaml:"vnl,omitempty" json:"vnl,omitempty"`
	SwitchPortDeprecated uint16 `yaml:"switchPort,omitempty" json:"switchPort,omitempty"` // VDE Switch port, not TCP/UDP port (only used by VDE networking)
	MACAddress           string `yaml:"macAddress,omitempty" json:"macAddress,omitempty"`
	Interface            string `yaml:"interface,omitempty" json:"interface,omitempty"`
	Metric               uint32 `yaml:"metric,omitempty" json:"metric,omitempty"`
}

type ProvisionMode = string

const (
	ProvisionModeSystem     ProvisionMode = "system"
	ProvisionModeUser       ProvisionMode = "user"
	ProvisionModeBoot       ProvisionMode = "boot"
	ProvisionModeDependency ProvisionMode = "dependency"
)

type Provision struct {
	Mode           ProvisionMode `yaml:"mode" json:"mode"` // default: "system"
	Script         string        `yaml:"script" json:"script"`
	SkipResolution bool          `yaml:"skipDefaultDependencyResolution,omitempty" json:"skipDefaultDependencyResolution,omitempty"`
}

type NineP struct {
	SecurityModel   string `yaml:"securityModel,omitempty" json:"securityModel,omitempty"`
	ProtocolVersion string `yaml:"protocolVersion,omitempty" json:"protocolVersion,omitempty"`
	Msize           string `yaml:"msize,omitempty" json:"msize,omitempty"`
	Cache           string `yaml:"cache,omitempty" json:"cache,omitempty"`
}

type Rosetta struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	BinFmt  bool `yaml:"binfmt" json:"binfmt"`
}

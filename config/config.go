package config

import (
	"fmt"
	"net"

	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/osutil"
)

const (
	AppName    = "colima"
	envProfile = "COLIMA_PROFILE" // environment variable for profile name
)

// VersionInfo is the application version info.
type VersionInfo struct {
	Version  string
	Revision string
}

func AppVersion() VersionInfo { return VersionInfo{Version: appVersion, Revision: revision} }
func EnvProfile() string      { return osutil.EnvVar(envProfile).Val() }

var (
	appVersion = "development"
	revision   = "unknown"
)

// Config is the application config.
type Config struct {
	CPU      int               `yaml:"cpu,omitempty"`
	Disk     int               `yaml:"disk,omitempty"`
	RootDisk int               `yaml:"rootDisk,omitempty"`
	Memory   float32           `yaml:"memory,omitempty"`
	Arch     string            `yaml:"arch,omitempty"`
	CPUType  string            `yaml:"cpuType,omitempty"`
	Network  Network           `yaml:"network,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"` // environment variables
	Hostname string            `yaml:"hostname"`

	// SSH
	SSHPort      int  `yaml:"sshPort,omitempty"`
	ForwardAgent bool `yaml:"forwardAgent,omitempty"`
	SSHConfig    bool `yaml:"sshConfig,omitempty"` // config generation

	// VM
	VMType               string `yaml:"vmType,omitempty"`
	VZRosetta            bool   `yaml:"rosetta,omitempty"`
	Binfmt               *bool  `yaml:"binfmt,omitempty"`
	NestedVirtualization bool   `yaml:"nestedVirtualization,omitempty"`
	DiskImage            string `yaml:"diskImage,omitempty"`
	PortForwarder        string `yaml:"portForwarder,omitempty"` // "ssh", "grpc"

	// volume mounts
	Mounts       []Mount `yaml:"mounts,omitempty"`
	MountType    string  `yaml:"mountType,omitempty"`
	MountINotify bool    `yaml:"mountInotify,omitempty"`

	// Runtime is one of docker, containerd.
	Runtime         string `yaml:"runtime,omitempty"`
	ActivateRuntime *bool  `yaml:"autoActivate,omitempty"`

	// Kubernetes configuration
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`

	// Docker configuration
	Docker map[string]any `yaml:"docker,omitempty"`

	// provision scripts
	Provision []Provision `yaml:"provision,omitempty"`
}

// Kubernetes is kubernetes configuration
type Kubernetes struct {
	Enabled    bool     `yaml:"enabled"`
	Version    string   `yaml:"version"`
	K3sArgs    []string `yaml:"k3sArgs"`
	ListenPort int      `yaml:"listenPort,omitempty"`
}

// Network is VM network configuration
type Network struct {
	Address         bool              `yaml:"address"`
	DNSResolvers    []net.IP          `yaml:"dns"`
	DNSHosts        map[string]string `yaml:"dnsHosts"`
	HostAddresses   bool              `yaml:"hostAddresses"`
	Mode            string            `yaml:"mode"` // shared, bridged
	BridgeInterface string            `yaml:"interface"`
	PreferredRoute  bool              `yaml:"preferredRoute"`
}

// Mount is volume mount
type Mount struct {
	Location   string `yaml:"location"`
	MountPoint string `yaml:"mountPoint,omitempty"`
	Writable   bool   `yaml:"writable"`
}

type Provision struct {
	Mode   string `yaml:"mode"`
	Script string `yaml:"script"`
}

func (c Config) MountsOrDefault() []Mount {
	if len(c.Mounts) > 0 {
		return c.Mounts
	}

	return []Mount{
		{Location: util.HomeDir(), Writable: true},
	}
}

// AutoActivate returns if auto-activation of host client config is enabled.
func (c Config) AutoActivate() bool {
	if c.ActivateRuntime == nil {
		return true
	}
	return *c.ActivateRuntime
}

// Empty checks if the configuration is empty.
func (c Config) Empty() bool { return c.Runtime == "" } // this may be better but not really needed.

func (c Config) DriverLabel() string {
	if util.MacOS13OrNewer() && c.VMType == "vz" {
		return "macOS Virtualization.Framework"
	}
	return "QEMU"
}

// Disk is an instance disk size
type Disk int

// GiB returns the string represent of the disk in GiB.
func (d Disk) GiB() string { return fmt.Sprintf("%dGiB", d) }

// Int returns the disk size in bytes.
func (d Disk) Int() int64 { return 1024 * 1024 * 1024 * int64(d) }

// CtxKey returns the context key for config.
func CtxKey() any {
	return struct{ name string }{name: "colima_config"}
}

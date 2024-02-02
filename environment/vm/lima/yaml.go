package lima

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/vmnet"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

func newConf(ctx context.Context, conf config.Config) (l Config, err error) {
	l.Arch = environment.Arch(conf.Arch).Value()

	// VM type is qemu except in few scenarios
	l.VMType = QEMU

	sameArchitecture := environment.HostArch() == l.Arch

	// when vz is chosen and OS version supports it
	if util.MacOS13OrNewer() && conf.VMType == VZ && sameArchitecture {
		l.VMType = VZ

		// Rosetta is only available on M1
		if conf.VZRosetta && util.MacOS13OrNewerOnM1() {
			if util.RosettaRunning() {
				l.Rosetta.Enabled = true
				l.Rosetta.BinFmt = true
			} else {
				logrus.Warnln("Unable to enable Rosetta: Rosetta2 is not installed")
				logrus.Warnln("Run 'softwareupdate --install-rosetta' to install Rosetta2")
			}
		}
	}

	if conf.CPUType != "" && conf.CPUType != "host" {
		l.CPUType = map[environment.Arch]string{
			l.Arch: conf.CPUType,
		}
	}

	l.Images = append(l.Images,
		File{
			Arch:     environment.AARCH64,
			Location: "https://github.com/abiosoft/colima-core/releases/download/v0.6.8-2/ubuntu-23.10-minimal-cloudimg-arm64.qcow2",
			Digest:   "sha512:00e3339bdebd98c3e003570ffb3ad4b01630fe4fcecd15061d5d58e14c07b211c718ed20fa3e4cce227d3b1c59fd98241eaa3e9e2cdfa04acfe32b4bc385428c",
		},
		File{
			Arch:     environment.X8664,
			Location: "https://github.com/abiosoft/colima-core/releases/download/v0.6.8-2/ubuntu-23.10-minimal-cloudimg-amd64.qcow2",
			Digest:   "sha512:97a6df4d4f4d13267fc7989933da537071513b3175a5785728992a8a198e0ceaaa0eaa2a9a4b01beaf6b998cc20b5995dc701e5c6d9ba2f4308e4208a84215df",
		},
	)

	if conf.CPU > 0 {
		l.CPUs = &conf.CPU
	}
	if conf.Memory > 0 {
		l.Memory = fmt.Sprintf("%dGiB", conf.Memory)
	}
	if conf.Disk > 0 {
		l.Disk = fmt.Sprintf("%dGiB", conf.Disk)
	}
	l.SSH = SSH{LocalPort: 0, LoadDotSSHPubKeys: false, ForwardAgent: conf.ForwardAgent}
	l.Containerd = Containerd{System: false, User: false}

	l.DNS = conf.Network.DNSResolvers
	l.HostResolver.Enabled = len(conf.Network.DNSResolvers) == 0
	l.HostResolver.Hosts = conf.Network.DNSHosts
	if l.HostResolver.Hosts == nil {
		l.HostResolver.Hosts = make(map[string]string)
	}

	if _, ok := l.HostResolver.Hosts["host.docker.internal"]; !ok {
		l.HostResolver.Hosts["host.docker.internal"] = "host.lima.internal"
	}

	l.Env = conf.Env
	if l.Env == nil {
		l.Env = make(map[string]string)
	}

	// extra required provision commands
	{
		// fix inotify
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeSystem,
			Script: "sysctl -w fs.inotify.max_user_watches=1048576",
		})

		// add user to docker group
		// "sudo", "usermod", "-aG", "docker", user
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeDependency,
			Script: "groupadd -f docker && usermod -aG docker {{ .User }}",
		})

		// set hostname
		hostname := config.CurrentProfile().ID
		if conf.Hostname != "" {
			hostname = conf.Hostname
		}
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeSystem,
			Script: "hostnamectl set-hostname " + hostname,
		})

	}

	// network setup
	{
		l.Networks = append(l.Networks, Network{
			Lima: "user-v2",
		})

		reachableIPAddress := true
		if conf.Network.Address {
			if l.VMType == VZ {
				l.Networks = append(l.Networks, Network{
					VZNAT:     true,
					Interface: vmnet.NetInterface,
				})
			} else {
				reachableIPAddress, _ = ctx.Value(daemon.CtxKey(vmnet.Name)).(bool)

				// network is currently limited to macOS.
				if util.MacOS() && reachableIPAddress {
					if err := func() error {
						socketFile := vmnet.Info().Socket.File()
						// ensure the socket file exists
						if _, err := os.Stat(socketFile); err != nil {
							return fmt.Errorf("vmnet socket file not found: %w", err)
						}

						l.Networks = append(l.Networks, Network{
							Socket:    socketFile,
							Interface: vmnet.NetInterface,
						})

						return nil
					}(); err != nil {
						reachableIPAddress = false
						logrus.Warn(fmt.Errorf("error setting up reachable IP address: %w", err))
					}
				}
			}

			// disable ports 80 and 443 when k8s is enabled and there is a reachable IP address
			// to prevent ingress (traefik) from occupying relevant host ports.
			if reachableIPAddress && conf.Kubernetes.Enabled && !ingressDisabled(conf.Kubernetes.K3sArgs) {
				l.PortForwards = append(l.PortForwards,
					PortForward{
						GuestIP:           net.ParseIP("0.0.0.0"),
						GuestPort:         80,
						GuestIPMustBeZero: true,
						Ignore:            true,
						Proto:             TCP,
					},
					PortForward{
						GuestIP:           net.ParseIP("0.0.0.0"),
						GuestPort:         443,
						GuestIPMustBeZero: true,
						Ignore:            true,
						Proto:             TCP,
					},
				)
			}
		}
	}

	// ports and sockets
	{
		// docker socket
		if conf.Runtime == docker.Name {
			l.PortForwards = append(l.PortForwards,
				PortForward{
					GuestSocket: "/var/run/docker.sock",
					HostSocket:  docker.HostSocketFile(),
					Proto:       TCP,
				})
			if config.CurrentProfile().ShortName == "default" {
				// for backward compatibility, will be removed in future releases
				l.PortForwards = append(l.PortForwards,
					PortForward{
						GuestSocket: "/var/run/docker.sock",
						HostSocket:  docker.LegacyDefaultHostSocketFile(),
						Proto:       TCP,
					})
			}
		}

		// handle port forwarding to allow listening on 0.0.0.0
		// bind 0.0.0.0
		l.PortForwards = append(l.PortForwards,
			PortForward{
				GuestIPMustBeZero: true,
				GuestIP:           net.ParseIP("0.0.0.0"),
				GuestPortRange:    [2]int{1, 65535},
				HostIP:            net.ParseIP("0.0.0.0"),
				HostPortRange:     [2]int{1, 65535},
				Proto:             TCP,
			},
		)
		// bind 127.0.0.1
		l.PortForwards = append(l.PortForwards,
			PortForward{
				GuestIP:        net.ParseIP("127.0.0.1"),
				GuestPortRange: [2]int{1, 65535},
				HostIP:         net.ParseIP("127.0.0.1"),
				HostPortRange:  [2]int{1, 65535},
				Proto:          TCP,
			},
		)
	}

	switch strings.ToLower(conf.MountType) {
	case "ssh", "sshfs", "reversessh", "reverse-ssh", "reversesshfs", REVSSHFS:
		l.MountType = REVSSHFS
	default:
		if l.VMType == VZ {
			l.MountType = VIRTIOFS
		} else { // qemu
			l.MountType = NINEP
		}
	}

	// Ubuntu minimal cloud image does not bundle sshfs
	// if sshfs is used, add as a dependency
	if l.MountType == REVSSHFS {
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeDependency,
			Script: `which sshfs || apt install -y sshfs`,
		})
	}

	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeSystem,
		Script: "mount -a",
	})

	// trim mounted drive to recover disk space
	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeSystem,
		Script: `readlink /usr/sbin/fstrim || fstrim -a`,
	})

	if len(conf.Mounts) == 0 {
		l.Mounts = append(l.Mounts,
			Mount{Location: "~", Writable: true},
			Mount{Location: filepath.Join("/tmp", config.CurrentProfile().ID), Writable: true},
		)
	} else {
		// overlapping mounts are problematic in Lima https://github.com/lima-vm/lima/issues/302
		if err = checkOverlappingMounts(conf.Mounts); err != nil {
			err = fmt.Errorf("overlapping mounts not supported: %w", err)
			return
		}

		l.Mounts = append(l.Mounts, Mount{Location: config.CacheDir(), Writable: false})
		cacheOverlapFound := false

		for _, m := range conf.Mounts {
			var location, mountPoint string
			location, err = util.CleanPath(m.Location)
			if err != nil {
				return
			}
			mountPoint, err = util.CleanPath(m.MountPoint)
			if err != nil {
				return
			}

			mount := Mount{Location: location, MountPoint: mountPoint, Writable: m.Writable}

			l.Mounts = append(l.Mounts, mount)

			// check if cache directory has been mounted by other mounts, and remove cache directory from mounts
			if strings.HasPrefix(config.CacheDir(), location) && !cacheOverlapFound {
				l.Mounts = l.Mounts[1:]
				cacheOverlapFound = true
			}
		}
	}

	// provision scripts
	for _, script := range conf.Provision {
		l.Provision = append(l.Provision, Provision{
			Mode:   script.Mode,
			Script: script.Script,
		})
	}

	return
}

type Arch = environment.Arch

// Config is lima config. Code copied from lima and modified.
type Config struct {
	VMType       VMType            `yaml:"vmType,omitempty" json:"vmType,omitempty"`
	Arch         Arch              `yaml:"arch,omitempty"`
	Images       []File            `yaml:"images"`
	CPUs         *int              `yaml:"cpus,omitempty"`
	Memory       string            `yaml:"memory,omitempty"`
	Disk         string            `yaml:"disk,omitempty"`
	Mounts       []Mount           `yaml:"mounts,omitempty"`
	MountType    MountType         `yaml:"mountType,omitempty" json:"mountType,omitempty"`
	SSH          SSH               `yaml:"ssh"`
	Containerd   Containerd        `yaml:"containerd"`
	Env          map[string]string `yaml:"env,omitempty"`
	DNS          []net.IP          `yaml:"dns"`
	Firmware     Firmware          `yaml:"firmware"`
	HostResolver HostResolver      `yaml:"hostResolver"`
	PortForwards []PortForward     `yaml:"portForwards,omitempty"`
	Networks     []Network         `yaml:"networks,omitempty"`
	Provision    []Provision       `yaml:"provision,omitempty" json:"provision,omitempty"`
	CPUType      map[Arch]string   `yaml:"cpuType,omitempty" json:"cpuType,omitempty"`
	Rosetta      Rosetta           `yaml:"rosetta,omitempty" json:"rosetta,omitempty"`
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

func checkOverlappingMounts(mounts []config.Mount) error {
	for i := 0; i < len(mounts)-1; i++ {
		for j := i + 1; j < len(mounts); j++ {
			a, err := util.CleanPath(mounts[i].Location)
			if err != nil {
				return err
			}

			b, err := util.CleanPath(mounts[j].Location)
			if err != nil {
				return err
			}

			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				return fmt.Errorf("'%s' overlaps '%s'", a, b)
			}
		}
	}
	return nil
}

// disableHas checks if the provided feature is indeed found in the disable configuration slice.
func ingressDisabled(disableFlags []string) bool {
	disabled := func(s string) bool { return s == "traefik" || s == "ingress" }
	for i, f := range disableFlags {
		if f == "--disable" {
			if len(disableFlags)-1 <= i {
				return false
			}
			if disabled(disableFlags[i+1]) {
				return true
			}
			continue
		}
		str := strings.SplitN(f, "=", 2)
		if len(str) < 2 || str[0] != "--disable" {
			continue
		}
		if disabled(str[1]) {
			return true
		}
	}
	return false
}

package lima

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/gvproxy"
	"github.com/abiosoft/colima/daemon/process/vmnet"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
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
		File{Arch: environment.AARCH64, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.5.0-2/alpine-lima-clm-3.16.2-aarch64.iso", Digest: "sha512:06abfa8c9fd954f8bfe4ce226bf282dd06e9dfbcd09f57566bf6c20809beb5a3367415b515e0a65d6a1638ecfd3a3bb3fb6d654dee3d72164bd0279370448507"},
		File{Arch: environment.X8664, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.5.0-2/alpine-lima-clm-3.16.2-x86_64.iso", Digest: "sha512:e9e118498f3a0745574ffc3686105d2c9777f7142164fe50ee47909dabd504c80ac29aeb76f9582706173310d1488d3b6f0ee9018e2a6aadc28eefb7767b63ec"},
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
	if len(l.DNS) == 0 {
		gvProxyEnabled, _ := ctx.Value(daemon.CtxKey(gvproxy.Name)).(bool)
		if gvProxyEnabled {
			l.DNS = append(l.DNS, net.ParseIP(gvproxy.GatewayIP))
			l.HostResolver.Enabled = false
		}
		reachableIPAddress, _ := ctx.Value(daemon.CtxKey(vmnet.Name)).(bool)
		if reachableIPAddress {
			if gvProxyEnabled {
				l.DNS = append(l.DNS, net.ParseIP(vmnet.NetGateway))
			}
		}
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
			Mode:   ProvisionModeUser,
			Script: "sudo usermod -aG docker $USER",
		})

		// allow env vars propagation for services
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeSystem,
			Script: `grep -q "^rc_env_allow" /etc/rc.conf || echo 'rc_env_allow="*"' >> /etc/rc.conf`,
		})

	}

	// network setup
	{
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
			if reachableIPAddress && conf.Kubernetes.Enabled && !disableHas(conf.Kubernetes.Disable, "ingress") {
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

		// gvproxy is cross-platform but not needed on Linux as slirp is only erratic on macOS.
		gvProxyEnabled, _ := ctx.Value(daemon.CtxKey(gvproxy.Name)).(bool)
		if gvProxyEnabled && util.MacOS() {
			var values struct {
				Vmnet struct {
					Enabled   bool
					Interface string
				}
				GVProxy struct {
					Enabled    bool
					MacAddress string
					IPAddress  net.IP
					Gateway    net.IP
				}
			}

			if reachableIPAddress {
				values.Vmnet.Enabled = true
				values.Vmnet.Interface = vmnet.NetInterface
			}

			gvProxyEnabled, _ := ctx.Value(daemon.CtxKey(gvproxy.Name)).(bool)
			if gvProxyEnabled {
				values.GVProxy.Enabled = true
				values.GVProxy.MacAddress = strings.ToUpper(gvproxy.MacAddress())
				values.GVProxy.IPAddress = net.ParseIP(gvproxy.DeviceIP)
				values.GVProxy.Gateway = net.ParseIP(gvproxy.GatewayIP)

				if err := func() error {
					tpl, err := embedded.ReadString("network/ifaces.sh")
					if err != nil {
						return err
					}

					script, err := util.ParseTemplate(tpl, values)
					if err != nil {
						return fmt.Errorf("error parsing template for network script: %w", err)
					}

					l.Provision = append(l.Provision, Provision{
						Mode:   ProvisionModeSystem,
						Script: string(script),
					})

					return nil
				}(); err != nil {
					logrus.Warn(fmt.Errorf("error setting up gvproxy network: %w", err))
				}
			}
		}
	}

	// port forwarding

	if conf.Layer {
		port := util.RandomAvailablePort()
		// set port for future retrieval
		l.Env[limautil.LayerEnvVar] = strconv.Itoa(port)
		// forward port
		l.PortForwards = append(l.PortForwards,
			PortForward{
				GuestPort: 23,
				HostPort:  port,
			})
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

	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeSystem,
		Script: "mkmntdirs && mount -a",
	})

	// trim mounted drive to recover disk space
	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeSystem,
		Script: `readlink /sbin/fstrim || fstrim -a`,
	})

	// workaround for slow virtiofs https://github.com/drud/ddev/issues/4466#issuecomment-1361261185
	// TODO: remove when fixed upstream
	if l.MountType == VIRTIOFS {
		l.Provision = append(l.Provision, Provision{
			Mode:   ProvisionModeSystem,
			Script: `stat /sys/class/block/vda/queue/write_cache && echo 'write through' > /sys/class/block/vda/queue/write_cache`,
		})
	}

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
	SSH          SSH               `yaml:"ssh,omitempty"`
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
	LocalPort         int  `yaml:"localPort"`
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
	ProvisionModeSystem ProvisionMode = "system"
	ProvisionModeUser   ProvisionMode = "user"
)

type Provision struct {
	Mode   ProvisionMode `yaml:"mode" json:"mode"` // default: "system"
	Script string        `yaml:"script" json:"script"`
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
func disableHas(disable []string, feature string) bool {
	for _, f := range disable {
		if f == feature {
			return true
		}
	}
	return false
}

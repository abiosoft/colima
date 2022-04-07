package lima

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/vm/lima/network"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

func newConf(ctx context.Context, conf config.Config) (l Config, err error) {
	l.Arch = environment.Arch(conf.Arch).Value()

	if conf.CPUType != "" {
		l.CPUType = map[environment.Arch]string{
			l.Arch: conf.CPUType,
		}
	}

	l.Images = append(l.Images,
		File{Arch: environment.AARCH64, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.4.0-1/alpine-lima-clm-3.15.2-aarch64.iso", Digest: "sha512:2e03281f2596c19cb46dd073adb4d35804d85f118e46137cf935d7af385062f2ade29b9e8734e5f3cf3b39acab3ed3714b06686bf9fe07e99e13421c915e59ba"},
		File{Arch: environment.X8664, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.4.0-1/alpine-lima-clm-3.15.2-x86_64.iso", Digest: "sha512:c340b2519e3727a70c71ca462fb9d5ae1e4aa7ecbbe79d707a95d30a013a89972612d3751fae76bfe4124f0620e90cf879e3fe206962a8b27a16c99be8f21b2c"},
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
	if conf.ForwardAgent {
		l.SSH = SSH{LocalPort: 0, LoadDotSSHPubKeys: false, ForwardAgent: conf.ForwardAgent}
	}
	l.Containerd = Containerd{System: false, User: false}
	l.Firmware.LegacyBIOS = false

	l.DNS = conf.DNS

	networkEnabled, _ := ctx.Value(ctxKeyNetwork).(bool)

	// always use host resolver to generate Lima's default resolv.conf file
	// colima will override this in VM when custom DNS is set
	l.HostResolver.Enabled = true
	l.HostResolver.Hosts = map[string]string{
		"host.docker.internal": "host.lima.internal",
	}

	l.Env = conf.Env

	// perform mounts in fstab.
	// required for 9p (lima >=v0.10.0)
	// idempotent for sshfs (lima <v0.10.0)
	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeSystem,
		Script: "mkmntdirs && mount -a",
	})

	// add user to docker group
	// "sudo", "usermod", "-aG", "docker", user
	l.Provision = append(l.Provision, Provision{
		Mode:   ProvisionModeUser,
		Script: `sudo usermod -aG docker $USER`,
	})

	// networking on Lima is limited to macOS
	if util.MacOS() && networkEnabled && conf.Network.Address {
		// only set network settings if vmnet startup is successful
		if err := func() error {
			ptpFile := network.PTPFile()
			// ensure the ptp file exists
			if _, err := os.Stat(ptpFile); err != nil {
				return err
			}
			tpl, err := embedded.ReadString("network/dhcp.sh")
			if err != nil {
				return err
			}

			ifaceToDisable := "eth0"
			if conf.Network.UserMode {
				ifaceToDisable = network.VmnetIface
			}
			values := struct{ Interface string }{Interface: ifaceToDisable}
			dhcpScript, err := util.ParseTemplate(tpl, values)
			if err != nil {
				return err
			}

			l.Networks = append(l.Networks, Network{
				VNL:        ptpFile,
				SwitchPort: 65535, // this is fixed
				Interface:  network.VmnetIface,
			})

			// disable one of the default routes accordingly
			// credit: https://github.com/abiosoft/colima/issues/140#issuecomment-1072599309
			l.Provision = append(l.Provision, Provision{
				Mode:   ProvisionModeSystem,
				Script: string(dhcpScript),
			})
			return nil
		}(); err != nil {
			logrus.Warn(fmt.Errorf("error setting up network: %w", err))
		}
	}

	// disable ports 80 and 443 when k8s is enabled and there is a reachable IP address
	// to prevent ingress (traefik) from occupying relevant host ports.
	if networkEnabled && conf.Kubernetes.Enabled && conf.Kubernetes.Ingress {
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

	// port forwarding
	{
		// docker socket
		if conf.Runtime == docker.Name {
			l.PortForwards = append(l.PortForwards,
				PortForward{
					GuestSocket: "/var/run/docker.sock",
					HostSocket:  docker.HostSocketFile(),
					Proto:       TCP,
				})
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

	l.MountType = NINEP
	if len(conf.Mounts) == 0 {
		l.Mounts = append(l.Mounts,
			Mount{Location: "~", Writable: true},
			Mount{Location: filepath.Join("/tmp", config.Profile().ID), Writable: true},
		)
	} else {
		// overlapping mounts are problematic in Lima https://github.com/lima-vm/lima/issues/302
		if err = checkOverlappingMounts(conf.Mounts); err != nil {
			err = fmt.Errorf("overlapping mounts not supported: %w", err)
			return
		}

		l.Mounts = append(l.Mounts, Mount{Location: config.CacheDir(), Writable: false})
		cacheOverlapFound := false

		for _, v := range conf.Mounts {
			m := volumeMount(v)
			var location string
			location, err = m.Path()
			if err != nil {
				return
			}
			l.Mounts = append(l.Mounts, Mount{Location: location, Writable: m.Writable()})

			// check if cache directory has been mounted by other mounts, and remove cache directory from mounts
			if strings.HasPrefix(config.CacheDir(), location) && !cacheOverlapFound {
				l.Mounts = l.Mounts[1:]
				cacheOverlapFound = true
			}
		}
	}

	return
}

type Arch = environment.Arch

// Config is lima config. Code copied from lima and modified.
type Config struct {
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
	DNS          []net.IP          `yaml:"-"` // will be handled manually by colima
	Firmware     Firmware          `yaml:"firmware"`
	HostResolver HostResolver      `yaml:"hostResolver"`
	PortForwards []PortForward     `yaml:"portForwards,omitempty"`
	Networks     []Network         `yaml:"networks,omitempty"`
	Provision    []Provision       `yaml:"provision,omitempty" json:"provision,omitempty"`
	CPUType      map[Arch]string   `yaml:"cpuType,omitempty" json:"cpuType,omitempty"`
}

type File struct {
	Location string `yaml:"location"` // REQUIRED
	Arch     Arch   `yaml:"arch,omitempty"`
	Digest   string `yaml:"digest,omitempty"`
}

type Mount struct {
	Location string `yaml:"location"` // REQUIRED
	Writable bool   `yaml:"writable"`
	NineP    NineP  `yaml:"9p,omitempty" json:"9p,omitempty"`
}

type SSH struct {
	LocalPort int `yaml:"localPort"`
	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	// Default: true
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

type Proto = string

type MountType = string

const (
	TCP Proto = "tcp"

	REVSSHFS MountType = "reverse-sshfs"
	NINEP    MountType = "9p"
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
	// VNL is a Virtual Network Locator (https://github.com/rd235/vdeplug4/commit/089984200f447abb0e825eb45548b781ba1ebccd).
	// On macOS, only VDE2-compatible form (optionally with vde:// prefix) is supported.
	VNL        string `yaml:"vnl,omitempty" json:"vnl,omitempty"`
	SwitchPort uint16 `yaml:"switchPort,omitempty" json:"switchPort,omitempty"` // VDE Switch port, not TCP/UDP port (only used by VDE networking)
	Interface  string `yaml:"interface,omitempty" json:"interface,omitempty"`
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

type volumeMount string

func (v volumeMount) Writable() bool {
	str := strings.SplitN(string(v), ":", 2)
	return len(str) >= 2 && str[1] == "w"
}

func (v volumeMount) Path() (string, error) {
	split := strings.SplitN(string(v), ":", 2)
	str := os.ExpandEnv(split[0])

	if strings.HasPrefix(str, "~") {
		str = strings.Replace(str, "~", util.HomeDir(), 1)
	}

	str = filepath.Clean(str)
	if !filepath.IsAbs(str) {
		return "", fmt.Errorf("relative paths not supported for mount '%s'", string(v))
	}

	return strings.TrimSuffix(str, "/") + "/", nil
}

func checkOverlappingMounts(mounts []string) error {
	for i := 0; i < len(mounts)-1; i++ {
		for j := i + 1; j < len(mounts); j++ {
			a, err := volumeMount(mounts[i]).Path()
			if err != nil {
				return err
			}

			b, err := volumeMount(mounts[j]).Path()
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

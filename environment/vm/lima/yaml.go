package lima

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/util"
)

func newConf(conf config.Config) (l Config, err error) {
	l.Arch = environment.Arch(conf.VM.Arch).Value()

	l.Images = append(l.Images,
		File{Arch: environment.AARCH64, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.3.4/alpine-lima-clm-3.14.3-aarch64.iso", Digest: "sha512:9a964a8709e6813cfdfc4ac58a5c811de04aa6eb1a41c13fe9662b39a28af8222d2684e501637d6e294f3857328a46d69969c3e6b8868eb633478596733f168e"},
		File{Arch: environment.X8664, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.3.4/alpine-lima-clm-3.14.3-x86_64.iso", Digest: "sha512:20237c13bf6e4769d4b027934d7084d436f3b930f4d8289aaba85d1db3025d28f0eabf7ea4aba8adef48ce3de1de3616a72b2fa8b31e818a6d30f4904483b8d9"},
	)

	l.CPUs = conf.VM.CPU
	l.Memory = fmt.Sprintf("%dGiB", conf.VM.Memory)
	l.Disk = fmt.Sprintf("%dGiB", conf.VM.Disk)

	l.SSH = SSH{LocalPort: 0, LoadDotSSHPubKeys: false, ForwardAgent: conf.VM.ForwardAgent}
	l.Containerd = Containerd{System: false, User: false}
	l.Firmware.LegacyBIOS = false

	l.DNS = conf.VM.DNS
	// always use host resolver to generate Lima's default resolv.conf file
	// colima will override this in VM when custom DNS is set
	l.HostResolver.Enabled = true
	l.HostResolver.Hosts = map[string]string{
		"host.docker.internal": "host.lima.internal",
	}

	l.Env = map[string]string{}
	for k, v := range conf.VM.Env {
		l.Env[k] = v
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

	if len(conf.VM.Mounts) == 0 {
		l.Mounts = append(l.Mounts,
			Mount{Location: "~", Writable: true},
			Mount{Location: filepath.Join("/tmp", config.Profile().ID), Writable: true},
		)
	} else {
		// overlapping mounts are problematic in Lima https://github.com/lima-vm/lima/issues/302
		if err = checkOverlappingMounts(conf.VM.Mounts); err != nil {
			err = fmt.Errorf("overlapping mounts not supported: %w", err)
			return
		}

		l.Mounts = append(l.Mounts, Mount{Location: config.CacheDir(), Writable: false})
		cacheOverlapFound := false

		for _, v := range conf.VM.Mounts {
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

// Config is lima config. Code copied from lima and modified.
type Config struct {
	Arch         environment.Arch  `yaml:"arch,omitempty"`
	Images       []File            `yaml:"images"`
	CPUs         int               `yaml:"cpus,omitempty"`
	Memory       string            `yaml:"memory,omitempty"`
	Disk         string            `yaml:"disk,omitempty"`
	Mounts       []Mount           `yaml:"mounts,omitempty"`
	SSH          SSH               `yaml:"ssh,omitempty"`
	Containerd   Containerd        `yaml:"containerd"`
	Env          map[string]string `yaml:"env,omitempty"`
	DNS          []net.IP          `yaml:"-"` // will be handled manually by colima
	Firmware     Firmware          `yaml:"firmware"`
	HostResolver HostResolver      `yaml:"hostResolver"`
	PortForwards []PortForward     `yaml:"portForwards,omitempty"`
}

type File struct {
	Location string           `yaml:"location"` // REQUIRED
	Arch     environment.Arch `yaml:"arch,omitempty"`
	Digest   string           `yaml:"digest,omitempty"`
}

type Mount struct {
	Location string `yaml:"location"` // REQUIRED
	Writable bool   `yaml:"writable"`
}

type SSH struct {
	LocalPort int `yaml:"localPort,omitempty"`
	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	// Default: true
	LoadDotSSHPubKeys bool `yaml:"loadDotSSHPubKeys"`
	ForwardAgent      bool `yaml:"forwardAgent,omitempty"` // default: false
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

const (
	TCP Proto = "tcp"
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

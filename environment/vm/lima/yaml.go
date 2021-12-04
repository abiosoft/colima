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
		File{Arch: environment.AARCH64, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.3.0-01/alpine-lima-clm-3.14.3-aarch64.iso", Digest: "sha512:b32dfef85d84de341b7c41cb0ec212f0e9f89e12e030f0e6761b8cc5c22e00edd1fbcb343817794ae4d7cf5468e94201453864b71f4cf12d67455078ce9a77bb"},
		File{Arch: environment.X8664, Location: "https://github.com/abiosoft/alpine-lima/releases/download/colima-v0.3.0-01/alpine-lima-clm-3.14.3-x86_64.iso", Digest: "sha512:41ab375082cee4a5327b76c7ef11c62730174481cfa1f572c8ec18241114fb461b880f1e70db92bc7d73caa42e70118ef02485fc4404b34ba1962f79d2de2743"},
	)

	l.CPUs = conf.VM.CPU
	l.Memory = fmt.Sprintf("%dGiB", conf.VM.Memory)
	l.Disk = fmt.Sprintf("%dGiB", conf.VM.Disk)

	l.SSH = SSH{LocalPort: conf.VM.SSHPort, LoadDotSSHPubKeys: false, ForwardAgent: conf.VM.ForwardAgent}
	l.Containerd = Containerd{System: false, User: false}
	l.Firmware.LegacyBIOS = false

	l.DNS = conf.VM.DNS
	l.UseHostResolver = len(l.DNS) == 0 // use host resolver when no DNS is set

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
		l.PortForwards = append(l.PortForwards,
			PortForward{
				GuestIP:        net.ParseIP("127.0.0.1"),
				GuestPortRange: [2]int{1, 65535},
				HostIP:         conf.PortInterface,
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
	Arch            environment.Arch  `yaml:"arch,omitempty"`
	Images          []File            `yaml:"images"`
	CPUs            int               `yaml:"cpus,omitempty"`
	Memory          string            `yaml:"memory,omitempty"`
	Disk            string            `yaml:"disk,omitempty"`
	Mounts          []Mount           `yaml:"mounts,omitempty"`
	SSH             SSH               `yaml:"ssh,omitempty"`
	Containerd      Containerd        `yaml:"containerd"`
	Env             map[string]string `yaml:"env,omitempty"`
	DNS             []net.IP          `yaml:"-"` // will be handled manually by colima
	Firmware        Firmware          `yaml:"firmware"`
	UseHostResolver bool              `yaml:"useHostResolver"`
	PortForwards    []PortForward     `yaml:"portForwards,omitempty"`
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
	LocalPort int `yaml:"localPort,omitempty"` // REQUIRED
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
	GuestIP        net.IP `yaml:"guestIP,omitempty" json:"guestIP,omitempty"`
	GuestPort      int    `yaml:"guestPort,omitempty" json:"guestPort,omitempty"`
	GuestPortRange [2]int `yaml:"guestPortRange,omitempty" json:"guestPortRange,omitempty"`
	GuestSocket    string `yaml:"guestSocket,omitempty" json:"guestSocket,omitempty"`
	HostIP         net.IP `yaml:"hostIP,omitempty" json:"hostIP,omitempty"`
	HostPort       int    `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	HostPortRange  [2]int `yaml:"hostPortRange,omitempty" json:"hostPortRange,omitempty"`
	HostSocket     string `yaml:"hostSocket,omitempty" json:"hostSocket,omitempty"`
	Proto          Proto  `yaml:"proto,omitempty" json:"proto,omitempty"`
	Ignore         bool   `yaml:"ignore,omitempty" json:"ignore,omitempty"`
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

	return str, nil
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

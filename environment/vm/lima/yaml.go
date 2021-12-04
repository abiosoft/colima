package lima

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/podman"
	"github.com/abiosoft/colima/util"
)

const ()

func newConf(conf config.Config) (l Config, err error) {
	l.Arch = environment.Arch(conf.VM.Arch).Value()

	var (
		aarch64Image  = "https://github.com/lima-vm/alpine-lima/releases/download/v0.2.2/alpine-lima-std-3.14.3-aarch64.iso"
		aarch64Digest = "sha512:6ff651023fbc4ec56c437124392d29cfa8eb8fe6d34c0e797b85b21734a6629aec38226c298f475b9ed63bef7664d49ba1bd5adc667c621efd7aa43e7020cc27"
		x86_64Image   = "https://github.com/lima-vm/alpine-lima/releases/download/v0.2.2/alpine-lima-std-3.14.3-x86_64.iso"
		x86_64Digest  = "sha512:573964991fb135aac18e44c444c1c924cd6110d4c823e887451e134adbecd7abb98bb84d22872cec1c9ed5b2cd9d87f664817adb15938ca3a69a9a2c70d66837"
	)

	switch r := conf.Runtime; r {
	case podman.Name:
		aarch64Image = podman.AARCH64Image
		aarch64Digest = podman.AARCH64Digest
		x86_64Image = podman.X86_64Image
		x86_64Digest = podman.X86_64Digest

	case docker.Name:
		aarch64Image = docker.AARCH64Image
		aarch64Digest = docker.AARCH64Digest
		x86_64Image = docker.X86_64Image
		x86_64Digest = docker.X86_64Digest

	case containerd.Name:
		aarch64Image = containerd.AARCH64Image
		aarch64Digest = containerd.AARCH64Digest
		x86_64Image = containerd.X86_64Image
		x86_64Digest = containerd.X86_64Digest
	}

	// use containerd image if using kubernetes
	if conf.Kubernetes.Enabled {
		aarch64Image = containerd.AARCH64Image
		aarch64Digest = containerd.AARCH64Digest
		x86_64Image = containerd.X86_64Image
		x86_64Digest = containerd.X86_64Digest
	}

	l.Images = append(l.Images,
		File{
			Arch:     environment.AARCH64,
			Location: aarch64Image,
			Digest:   aarch64Digest,
		},
		File{
			Arch:     environment.X8664,
			Location: x86_64Image,
			Digest:   x86_64Digest,
		},
	)

	l.CPUs = conf.VM.CPU
	l.Memory = fmt.Sprintf("%dGiB", conf.VM.Memory)
	l.Disk = fmt.Sprintf("%dGiB", conf.VM.Disk)

	l.SSH = SSH{LocalPort: conf.VM.SSHPort, LoadDotSSHPubKeys: false, ForwardAgent: conf.VM.ForwardAgent}
	l.Containerd = Containerd{System: false, User: false}
	l.Firmware.LegacyBIOS = true

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

		// podman socket for docker compability
		if conf.Runtime == podman.Name {
			l.PortForwards = append(l.PortForwards,
				PortForward{
					GuestSocket: "/run/podman/podman.socket",
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

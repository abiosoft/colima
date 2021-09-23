package lima

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/container/containerd"
	"net"
	"path/filepath"
)

func newConf(conf config.Config) (l Config) {
	l.Arch = "default"

	l.Images = append(l.Images,
		File{Arch: X8664, Location: "https://cloud-images.ubuntu.com/hirsute/current/hirsute-server-cloudimg-amd64.img"},
		File{Arch: AARCH64, Location: "https://cloud-images.ubuntu.com/hirsute/current/hirsute-server-cloudimg-arm64.img"})

	l.CPUs = conf.VM.CPU
	l.Memory = fmt.Sprintf("%dGiB", conf.VM.Memory)
	l.Disk = fmt.Sprintf("%dGiB", conf.VM.Disk)

	l.Mounts = append(l.Mounts,
		Mount{Location: "~", Writable: false},
		Mount{Location: filepath.Join("/tmp", config.AppName()), Writable: true},
	)

	l.SSH = SSH{LocalPort: config.SSHPort(), LoadDotSSHPubKeys: false}
	l.Containerd = Containerd{System: conf.Runtime == containerd.Name, User: false}
	l.Firmware.LegacyBIOS = false

	l.DNS = conf.VM.DNS

	l.Env = map[string]string{}
	for k, v := range conf.VM.Env {
		l.Env[k] = v
	}

	return
}

// Config is lima config. Code copied from lima and modified.
type Config struct {
	Arch       Arch              `yaml:"arch,omitempty"`
	Images     []File            `yaml:"images"`
	CPUs       int               `yaml:"cpus,omitempty"`
	Memory     string            `yaml:"memory,omitempty"`
	Disk       string            `yaml:"disk,omitempty"`
	Mounts     []Mount           `yaml:"mounts,omitempty"`
	SSH        SSH               `yaml:"ssh,omitempty"`
	Containerd Containerd        `yaml:"containerd"`
	Env        map[string]string `yaml:"env,omitempty"`
	DNS        []net.IP          `yaml:"-"` // will be handled manually by colima
	Firmware   Firmware          `yaml:"firmware"`
}

type Arch = string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
)

type File struct {
	Location string `yaml:"location"` // REQUIRED
	Arch     Arch   `yaml:"arch,omitempty"`
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

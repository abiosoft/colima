package lima

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/vmnet"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

func newConf(ctx context.Context, conf config.Config) (l limaconfig.Config, err error) {
	l.Arch = environment.Arch(conf.Arch).Value()

	// VM type is qemu except in few scenarios
	l.VMType = limaconfig.QEMU

	sameArchitecture := environment.HostArch() == l.Arch

	// when vz is chosen and OS version supports it
	if util.MacOS13OrNewer() && conf.VMType == limaconfig.VZ && sameArchitecture {
		l.VMType = limaconfig.VZ

		// Rosetta is only available on Apple Silicon
		if conf.VZRosetta && util.MacOS13OrNewerOnArm() {
			if util.RosettaRunning() {
				l.VMOpts.VZOpts.Rosetta.Enabled = true
				l.VMOpts.VZOpts.Rosetta.BinFmt = true
			} else {
				logrus.Warnln("Unable to enable Rosetta: Rosetta2 is not installed")
				logrus.Warnln("Run 'softwareupdate --install-rosetta' to install Rosetta2")
			}
		}

		if util.MacOSNestedVirtualizationSupported() {
			l.NestedVirtualization = conf.NestedVirtualization
		}
	}

	// when krunkit is chosen and OS version supports it
	if util.MacOS13OrNewerOnArm() && conf.VMType == limaconfig.Krunkit && sameArchitecture {
		l.VMType = limaconfig.Krunkit
	}

	if conf.CPUType != "" && conf.CPUType != "host" {
		l.VMOpts.QEMU.CPUType = map[environment.Arch]string{
			l.Arch: conf.CPUType,
		}
	}

	if conf.CPU > 0 {
		l.CPUs = &conf.CPU
	}
	if conf.Memory > 0 {
		l.Memory = fmt.Sprintf("%dMiB", uint32(conf.Memory*1024))
	}
	if conf.RootDisk > 0 {
		l.Disk = fmt.Sprintf("%dGiB", conf.RootDisk)
	}
	l.SSH = limaconfig.SSH{LocalPort: conf.SSHPort, LoadDotSSHPubKeys: false, ForwardAgent: conf.ForwardAgent}
	l.Containerd = limaconfig.Containerd{System: false, User: false}

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
		l.Provision = append(l.Provision, limaconfig.Provision{
			Mode:   limaconfig.ProvisionModeSystem,
			Script: "sysctl -w fs.inotify.max_user_watches=1048576",
		})

		// add user to docker group
		// "sudo", "usermod", "-aG", "docker", user
		if conf.Runtime == docker.Name {
			l.Provision = append(l.Provision, limaconfig.Provision{
				Mode:   limaconfig.ProvisionModeDependency,
				Script: "groupadd -f docker && usermod -aG docker {{ .User }}",
			})
		}

		// add user to incus-admin group
		// "sudo", "usermod", "-aG", "incus-admin", user
		if conf.Runtime == incus.Name {
			l.Provision = append(l.Provision, limaconfig.Provision{
				Mode:   limaconfig.ProvisionModeDependency,
				Script: "groupadd -f incus-admin && usermod -aG incus-admin {{ .User }}",
			})
		}

		// set hostname
		hostname := config.CurrentProfile().ID
		if conf.Hostname != "" {
			hostname = conf.Hostname
		}
		l.Provision = append(l.Provision, limaconfig.Provision{
			Mode:   limaconfig.ProvisionModeSystem,
			Script: "grep '127.0.0.1 " + hostname + "' /etc/hosts || echo '127.0.0.1 " + hostname + "' >> /etc/hosts",
		})
		l.Provision = append(l.Provision, limaconfig.Provision{
			Mode:   limaconfig.ProvisionModeSystem,
			Script: "hostnamectl set-hostname " + hostname,
		})

	}

	// network setup
	{
		l.Networks = append(l.Networks, limaconfig.Network{
			Lima: "user-v2",
		})

		reachableIPAddress := true
		if conf.Network.Address {
			metric := limautil.NetMetric
			if conf.Network.PreferredRoute {
				metric = limautil.NetMetricPreferred
			}
			// vmnet is used for bridged mode, otherwise VZ uses VZNAT
			if l.VMType == limaconfig.VZ && conf.Network.Mode != "bridged" {
				l.Networks = append(l.Networks, limaconfig.Network{
					VZNAT:     true,
					Interface: limautil.NetInterface,
					Metric:    metric,
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

						l.Networks = append(l.Networks, limaconfig.Network{
							Socket:    socketFile,
							Interface: limautil.NetInterface,
							Metric:    metric,
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
					limaconfig.PortForward{
						GuestIP:           net.IPv4zero,
						GuestPort:         80,
						GuestIPMustBeZero: true,
						Ignore:            true,
						Proto:             limaconfig.TCP,
					},
					limaconfig.PortForward{
						GuestIP:           net.IPv4zero,
						GuestPort:         443,
						GuestIPMustBeZero: true,
						Ignore:            true,
						Proto:             limaconfig.TCP,
					},
				)
			}

			// disable port forwarding for Incus when there is a reachable IP address for consistent behaviour
			if reachableIPAddress && conf.Runtime == incus.Name {
				l.PortForwards = append(l.PortForwards,
					limaconfig.PortForward{
						GuestIP:           net.IPv4zero,
						GuestIPMustBeZero: true,
						GuestPortRange:    [2]int{1, 65535},
						HostPortRange:     [2]int{1, 65535},
						Ignore:            true,
						Proto:             limaconfig.TCP,
					},
					limaconfig.PortForward{
						GuestIP:        net.ParseIP("127.0.0.1"),
						GuestPortRange: [2]int{1, 65535},
						HostPortRange:  [2]int{1, 65535},
						Ignore:         true,
						Proto:          limaconfig.TCP,
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
				limaconfig.PortForward{
					GuestSocket: "/var/run/docker.sock",
					HostSocket:  docker.HostSocketFile(),
					Proto:       limaconfig.TCP,
				},
				limaconfig.PortForward{
					GuestSocket: "/var/run/containerd/containerd.sock",
					HostSocket:  containerd.HostSocketFiles().Containerd,
					Proto:       limaconfig.TCP,
				})

			if config.CurrentProfile().ShortName == "default" {
				// for backward compatibility, will be removed in future releases
				l.PortForwards = append(l.PortForwards,
					limaconfig.PortForward{
						GuestSocket: "/var/run/docker.sock",
						HostSocket:  docker.LegacyDefaultHostSocketFile(),
						Proto:       limaconfig.TCP,
					})
			}
		}

		// containerd socket
		if conf.Runtime == containerd.Name {
			l.PortForwards = append(l.PortForwards,
				limaconfig.PortForward{
					GuestSocket: "/var/run/containerd/containerd.sock",
					HostSocket:  containerd.HostSocketFiles().Containerd,
					Proto:       limaconfig.TCP,
				},
				limaconfig.PortForward{
					GuestSocket: "/var/run/buildkit/buildkitd.sock",
					HostSocket:  containerd.HostSocketFiles().Buildkitd,
					Proto:       limaconfig.TCP,
				})
		}

		// incus socket
		if conf.Runtime == incus.Name {
			l.PortForwards = append(l.PortForwards,
				limaconfig.PortForward{
					GuestSocket: "/var/lib/incus/unix.socket",
					HostSocket:  incus.HostSocketFile(),
					Proto:       limaconfig.TCP,
				})
		}

		if conf.PortForwarder == "none" {
			// disable port forwarding
			l.PortForwards = append(l.PortForwards,
				limaconfig.PortForward{
					GuestIP: net.IPv4zero,
					Proto:   "any",
					Ignore:  true,
				})
		} else {
			// handle port forwarding to allow listening on 0.0.0.0
			// bind 0.0.0.0
			l.PortForwards = append(l.PortForwards,
				limaconfig.PortForward{
					GuestIPMustBeZero: true,
					GuestIP:           net.IPv4zero,
					GuestPortRange:    [2]int{1, 65535},
					HostIP:            net.IPv4zero,
					HostPortRange:     [2]int{1, 65535},
					Proto:             limaconfig.TCP,
				},
				limaconfig.PortForward{
					GuestIPMustBeZero: true,
					GuestIP:           net.IPv4zero,
					GuestPortRange:    [2]int{1, 65535},
					HostIP:            net.IPv4zero,
					HostPortRange:     [2]int{1, 65535},
					Proto:             limaconfig.UDP,
				},
			)
			// bind 127.0.0.1
			l.PortForwards = append(l.PortForwards,
				limaconfig.PortForward{
					GuestIP:        net.ParseIP("127.0.0.1"),
					GuestPortRange: [2]int{1, 65535},
					HostIP:         net.ParseIP("127.0.0.1"),
					HostPortRange:  [2]int{1, 65535},
					Proto:          limaconfig.TCP,
				},
				limaconfig.PortForward{
					GuestIP:        net.ParseIP("127.0.0.1"),
					GuestPortRange: [2]int{1, 65535},
					HostIP:         net.ParseIP("127.0.0.1"),
					HostPortRange:  [2]int{1, 65535},
					Proto:          limaconfig.UDP,
				},
			)

			// bind all host addresses when network address is not enabled
			if !conf.Network.Address && conf.Network.HostAddresses {
				for _, ip := range util.HostIPAddresses() {
					l.PortForwards = append(l.PortForwards,
						limaconfig.PortForward{
							GuestIP:        ip,
							GuestPortRange: [2]int{1, 65535},
							HostIP:         ip,
							HostPortRange:  [2]int{1, 65535},
							Proto:          limaconfig.TCP,
						},
					)
				}
			}
		}
	}

	switch strings.ToLower(conf.MountType) {
	case "ssh", "sshfs", "reversessh", "reverse-ssh", "reversesshfs", limaconfig.REVSSHFS:
		l.MountType = limaconfig.REVSSHFS
	default:
		if l.VMType == limaconfig.VZ {
			l.MountType = limaconfig.VIRTIOFS
		} else { // qemu
			l.MountType = limaconfig.NINEP
		}
	}

	/*
		provision scripts for disk actions
	*/

	// ensure all volumes are mounted.
	l.Provision = append(l.Provision, limaconfig.Provision{
		Mode:   limaconfig.ProvisionModeSystem,
		Script: "mount -a",
	})

	// trim mounted drive to recover disk space
	// however problematic for incus
	if conf.Runtime != incus.Name {
		l.Provision = append(l.Provision, limaconfig.Provision{
			Mode:   limaconfig.ProvisionModeSystem,
			Script: `readlink /usr/sbin/fstrim || fstrim -a`,
		})
	}

	// grow partition in case disk size has increased
	l.Provision = append(l.Provision, limaconfig.Provision{
		Mode:   limaconfig.ProvisionModeSystem,
		Script: "resize2fs " + diskByLabelPath(config.CurrentProfile().ID) + " || true",
	})

	/* end */

	if conf.Mounts != nil && len(conf.Mounts) == 0 {
		l.Mounts = append(l.Mounts,
			limaconfig.Mount{Location: "~", Writable: true},
		)
	} else {
		// overlapping mounts are problematic in Lima https://github.com/lima-vm/lima/issues/302
		if err = checkOverlappingMounts(conf.Mounts); err != nil {
			err = fmt.Errorf("overlapping mounts not supported: %w", err)
			return
		}

		l.Mounts = append(l.Mounts, limaconfig.Mount{Location: config.CacheDir(), Writable: false})
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

			mount := limaconfig.Mount{Location: location, MountPoint: mountPoint, Writable: m.Writable}

			l.Mounts = append(l.Mounts, mount)

			// check if cache directory has been mounted by other mounts, and remove cache directory from mounts
			if strings.HasPrefix(config.CacheDir(), location) && !cacheOverlapFound {
				l.Mounts = l.Mounts[1:]
				cacheOverlapFound = true
			}
		}
	}

	// provision scripts (only pass Lima-managed modes)
	for _, script := range conf.Provision {
		if script.IsColimaMode() {
			continue
		}
		l.Provision = append(l.Provision, limaconfig.Provision{
			Mode:   script.Mode,
			Script: script.Script,
		})
	}

	return
}

type Arch = environment.Arch

func selectPath(m config.Mount) (string, error) {
	if m.MountPoint != "" {
		return util.CleanPath(m.MountPoint)
	}

	return util.CleanPath(m.Location)
}

func checkOverlappingMounts(mounts []config.Mount) error {
	for i := 0; i < len(mounts)-1; i++ {
		a, err := selectPath(mounts[i])
		if err != nil {
			return err
		}
		for j := i + 1; j < len(mounts); j++ {
			b, err := selectPath(mounts[j])
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

const diskLabelMaxLength = 16 // https://tldp.org/HOWTO/Partition/labels.html

func diskByLabelPath(instanceId string) string {
	name := "lima-" + instanceId
	if len(name) > diskLabelMaxLength {
		name = name[:diskLabelMaxLength]
	}

	return "/dev/disk/by-label/" + name
}

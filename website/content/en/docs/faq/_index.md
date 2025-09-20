---
title: FAQs
weight: 6
---

<!-- doctoc: https://github.com/thlorenz/doctoc -->

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Generic](#generic)
  - ["How does Colima work?"](#how-does-colima-work)
  - ["What's my login password?"](#whats-my-login-password)
  - ["Does Colima work on ARM Mac?"](#does-colima-work-on-arm-mac)
  - ["Can I run non-Ubuntu guests?"](#can-i-run-non-ubuntu-guests)
  - ["Can I run other container engines such as Docker and Podman? What about Kubernetes?"](#can-i-run-other-container-engines-such-as-docker-and-podman-what-about-kubernetes)
  - ["Can I run Colima with a remote Linux machine?"](#can-i-run-colima-with-a-remote-linux-machine)
  - ["Advantages compared to Docker Desktop?"](#advantages-compared-to-docker-desktop)
- [Networking](#networking)
  - ["Cannot access the guest IP 192.168.5.15 from the host"](#cannot-access-the-guest-ip-192168515-from-the-host)
  - ["Ping shows duplicate packets and massive response times"](#ping-shows-duplicate-packets-and-massive-response-times)
  - ["IP address is not assigned for vmnet networks"](#ip-address-is-not-assigned-for-vmnet-networks)

### Generic

#### "How does Colima work?"

Colima uses [Lima](https://github.com/lima-vm/lima) under the hood to provide Linux virtual machines on macOS and Linux.

- Hypervisor: [QEMU (default on Linux), or Virtualization.framework (default on macOS)](../config/vmtype/)
- Filesystem sharing: [Reverse SSHFS, virtio-9p-pci aka virtfs (default for QEMU), or virtiofs (default for Virtualization.framework)](../config/mount/)
- Port forwarding: [`ssh -L`](../config/port), automated by watching `/proc/net/tcp` and `iptables` events in the guest
- Container runtime: Docker (default), containerd, or Podman

#### "What's my login password?"

Password is disabled and locked by default.
You have to use `colima ssh` to open a shell.

Alternatively, you may also directly ssh into the guest using the generated SSH config.
The connection details can be inspected by running `colima status`.

#### "Does Colima work on ARM Mac?"

Yes, Colima fully supports Apple Silicon (ARM) Macs.

#### "Can I run non-Ubuntu guests?"

AlmaLinux, Alpine, Arch Linux, Debian, Fedora, openSUSE, Oracle Linux, and Rocky are also known to work.
{{% fixlinks %}}
See [`./templates/`](./templates/).
{{% /fixlinks %}}

An image has to satisfy the following requirements:

- systemd or OpenRC
- cloud-init
- The following binaries to be preinstalled:
  - `sudo`
- The following binaries to be preinstalled, or installable via the package manager:
  - `sshfs`
  - `newuidmap` and `newgidmap`

#### "Can I run other container engines such as Docker and Podman? What about Kubernetes?"

{{% fixlinks %}}
Yes, Colima supports multiple container runtimes:

- **Docker** (default): `colima start`
- **containerd**: `colima start --runtime containerd`
- **Incus**: `colima start --runtime incus`

**Kubernetes support:**

- **k3s**: `colima start --kubernetes`

See also related projects:

- [Rancher Desktop](https://rancherdesktop.io/): Kubernetes and container management to the desktop
- [Lima](https://github.com/lima-vm/lima): The underlying virtualization technology
- [Podman Desktop](https://podman-desktop.io/): Containers and Kubernetes for application developers

{{% /fixlinks %}}

#### "Can I run Colima with a remote Linux machine?"

Colima is designed for local development. For remote Linux machines, you can use [sshocker](https://github.com/lima-vm/sshocker) or regular Docker over SSH.

#### "Advantages compared to Docker Desktop?"

- **Free and open source**: Colima is completely free (MIT License)
- **Lightweight**: Lower resource consumption compared to Docker Desktop
- **No licensing restrictions**: No commercial use limitations
- **Simple CLI**: Easy to use command-line interface
- **Lima-powered**: Built on proven open-source virtualization technology

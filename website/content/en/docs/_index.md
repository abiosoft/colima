---
title: "Colima: Container Runtimes on macOS"
linkTitle: Documentation
menu: { main: { weight: 20 } }
weight: 20
---

{{% fixlinks %}}
Colima provides container runtimes on macOS (and Linux) with minimal setup.

✅ Simple CLI to start/stop container runtime

✅ Built-in support for [Docker](https://www.docker.com) (default)

✅ Support for [containerd](https://containerd.io) and [Kubernetes](https://kubernetes.io)

✅ Automatic file sharing

✅ Automatic port forwarding

✅ Intel on Intel

✅ ARM on ARM

✅ Various guest Linux distributions: [AlmaLinux](./templates/almalinux.yaml), [Alpine](./templates/alpine.yaml), [Arch Linux](./templates/archlinux.yaml), [Debian](./templates/debian.yaml), [Fedora](./templates/fedora.yaml), [openSUSE](./templates/opensuse.yaml), [Oracle Linux](./templates/oraclelinux.yaml), [Rocky](./templates/rocky.yaml), [Ubuntu](./templates/ubuntu.yaml) (default), ...

Related project: [sshocker (ssh with file sharing and port forwarding)](https://github.com/lima-vm/sshocker)

## Motivation

Colima was created to provide a simple and easy-to-use alternative for running Docker Desktop on macOS.
It leverages [Lima](https://github.com/lima-vm/lima) to provide Linux virtual machines with automatic file sharing and port forwarding.
The goal is to provide Docker (and Kubernetes) on macOS with minimal setup, while also supporting containerd and other container runtimes.
{{% /fixlinks %}}

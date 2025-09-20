---
title: Lima (underlying technology)
weight: 0
---

## "How does Colima relate to Lima?"

Colima is built on top of [Lima](https://github.com/lima-vm/lima), which provides Linux virtual machines on macOS and Linux with automatic file sharing and port forwarding.

Lima is the underlying virtualization technology that makes Colima possible.
Colima provides a simplified interface specifically focused on container runtimes (Docker, containerd) and Kubernetes.

| Feature            | Lima                              | Colima                              |
|----------------------|-----------------------------------|-------------------------------------|
| containerd           | `limactl start`                   | `colima start --runtime=containerd` |
| Docker               | `limactl start template://docker` | `colima start`                      |
| Kubernetes (k3s)     | `limactl start template://k3s`    | `colima start --kubernetes`         |

## Key advantages of Colima over raw Lima

- **Simplified CLI**: Single `colima start` command vs complex Lima templates
- **Docker-first**: Docker works out of the box without configuration
- **Better defaults**: Optimized for container workloads
- **Integrated Kubernetes**: Built-in k3s support with `--kubernetes` flag
- **Automatic cleanup**: Better resource management and cleanup

The `colima` CLI provides a more streamlined experience compared to `limactl`:

| Configuration      | Lima                                       | Colima                            |
|--------------------|--------------------------------------------|-----------------------------------|
| CPUs               | `limactl start --cpus=4`                   | `colima start --cpu=4`            |
| Memory             | Complex YAML configuration                 | `colima start --memory=8`        |
| Runtime selection  | `limactl start template://docker`          | `colima start --runtime=docker`  |
| Kubernetes         | `limactl start template://k3s`             | `colima start --kubernetes`      |

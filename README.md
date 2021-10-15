# Colima

[![Go](https://github.com/abiosoft/colima/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/go.yml)

Container runtimes on macOS with minimal setup.

![Demonstration](colima.gif)

## Features

- Simple CLI interface
- Docker and Containerd support
- Port Forwarding
- Volume mounts
- Kubernetes

## Getting Started

### Prerequisites

Colima requires [Lima](https://github.com/lima-vm/lima), Docker client (for Docker runtime) and kubectl (if Kubernetes
will be enabled).

```
brew install lima docker kubectl
```

### Installation

```
curl -LO https://github.com/abiosoft/colima/releases/download/v0.2.2/colima-amd64 && sudo install colima-amd64 /usr/local/bin/colima
```

Verify install

```
colima version
```

Command line usages

```
colima --help
colima start --help
```

### Building from Source

Requires [Go](https://golang.org).

```sh
# clone repo and cd into it
git clone https://github.com/abiosoft/colima
cd colima

make install # or `sudo make install` if /usr/local/bin requires root
```

## Usage

### Docker

`colima start` starts and setup Docker by default. You can use the `docker` client on macOS after `colima start` with no
additional setup.

### Containerd

`colima start --runtime containerd` starts and setup Containerd. You can use `colima nerdctl` to interact with
Containerd using [nerdctl](https://github.com/containerd/nerdctl).

It is recommended to run `colima nerdctl install` to install `nerdctl` alias script in $PATH.

### Kubernetes

To enable Kubernetes, start Colima with `--with-kubernetes` flag.

```
colima start --with-kubernetes
```

#### Interacting with Image Registry

For Docker runtime, images built or pulled with Docker are accessible to Kubernetes.

For Containerd runtime, images built or pulled in the `k8s.io` namespace are accessible to Kubernetes.

### Customizing the VM

The default VM created by Colima has 2 CPUs, 2GiB memory and 60GiB storage.

The VM can be customized by passing `--cpu`, `--memory` and `--disk` to `colima start`. If VM is already created, stop
the VM and apply the flags when starting it.

**NOTE** that only cpu and memory can be changed at anytime. Disk size cannot be changed after the VM is created.

#### Customization Examples

- create VM with 1CPU, 2GiB memory and 10GiB storage.

  ```
  colima start --cpu 1 --memory 2 --disk 10
  ```

- modify an existing VM to 4CPUs and 8GiB memory.

  ```
  colima stop
  colima start --cpu 4 --memory 8
  ```

## Project Goal

To provide container runtimes on macOS with minimal setup.

## What is with the name?

Colima means Containers in Lima.

Since Lima is aka Linux on Mac. By transitivity, Colima can also mean Containers on Linux on Mac.

## FAQ

<details>
<summary>Can it run alongside Docker for Mac?</summary>
<p>

No, except when started with Containerd runtime. Colima assumes to be the default Docker context and will conflict with
Docker for Mac. You should run either, not both.

</p>
</details>

<details>
<summary>How to enable writable volumes?</summary>
<p>

By default, Colima mounts the host's $HOME directory as readonly in the VM. Volume mounts and Compose should work as
expected but only readonly.

Colima uses Lima for the VM and Lima's support for writable volumes is still experimental. It is recommended to only
mount the necessary directories as writable rather than the entire $HOME directory.

The following mounts $HOME/projects and $HOME/work directories as writable.

```
colima start --mount $HOME/projects:w --mount $HOME/work:w
```

</p>
</details>

<details>
<summary>How to customize Docker config e.g. add insecure registries?</summary>
<p>

On first startup, Colima generates Docker daemon.json file at `$HOME/.colima/docker/daemon.json`.

Simply modify the daemon.json file accordingly and restart Colima.

</p>
</details>

<details>
<summary>How does it compare to minikube, Kind, K3d?</summary>
<p>

### For Kubernetes

Yes, you can create a Kubernetes cluster with minikube (with Docker driver), Kind or K3d instead of enabling Kubernetes
in Colima. Those are better options if you need multiple clusters, or do not need Docker and Kubernetes to share the
same images and runtime.

### For Docker

Minikube with Docker runtime can expose the cluster's Docker with `minikube docker-env`. But there are some caveats.

- Kubernetes is not optional, even if you only need Docker.

- All of minikube's free drivers for macOS fall-short in one of performance, port forwarding or volumes. While
  port-forwarding and volumes are non-issue for Kubernetes, they can be a deal breaker for Docker-only use.

</p>
</details>


<details>
<summary>Are M1 macs supported?</summary>
<p>

Colima is written to support M1 macs but not tested, as the author do not currently possess an M1 device.

</p>
</details>

## Help Wanted

- Homebrew formula
- Documentation page
- Testing on M1 Macs

## License

MIT

# Colima

Docker (and Kubernetes) on macOS with minimal setup.

## Getting Started

### Prerequisites

Colima requires [Lima](https://github.com/lima-vm/lima) and Docker client (and kubectl if Kubernetes will be enabled).

```
brew install lima docker kubectl
```

### Installation

```
curl -LO https://raw.githubusercontent.com/abiosoft/colima/main/colima && sudo install colima /usr/local/bin/colima
```

Verify install

```sh
colima version
```

Command line usage

```
colima --help
```

## Usage

### Docker

`colima start` starts and setup Docker by default.
You can use the `docker` client on macOS after `colima start` with no additional setup.

### Kubernetes

To enable Kubernetes, start Colima with `--with-kubernetes` flag.
Colima uses minikube in background which requires at least 2CPUs and ~2.2GiB memory to run.

Colima's Docker runtime is used for Kubernetes. Therefore, images built or pulled with Docker are accessible to Kubernetes.

```
colima start --with-kubernetes
```

### Customizing the VM

The default VM created by Colima has 2 CPUs, 4GiB memory and 60GiB storage.

The VM can be customized by passing `--cpu`, `--memory` and `--disk` to `colima start`.
If VM is already created, stop the VM and apply the flags when starting it.

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

The current version is in usable alpha state and provides Docker and Kubernetes using Docker runtime.
If there is enough interest in the project, the plan is a rewrite in Go with added flexibility to
support other container runtimes (e.g. containerd/nerdctl, crio/podman).

## What is with the name?

Colima means COntainers in LIMA

## FAQ

<details>
<summary>Can it run alongside Docker for Mac?</summary>
<p>

No. Colima assumes to be the default Docker context and will conflict with Docker for Mac. You should either, not both.

</p>
</details>

<details>
<summary>What about Docker Volumes and Docker Compose?</summary>
<p>

Colima mounts the host's $HOME directory as readonly in the VM. Volume mounts and docker compose should work as expected but only readonly.

Colima uses Lima for the VM and Lima's support for writeable volumes is still experimental.
Volumes are thereby made readonly in Colima for now.

</p>
</details>

<details>
<summary>How can I enable verbose output?</summary>
<p>

The log file is at $HOME/.colima/out.log, you can simply tail it.

```
tail -f $HOME/.colima/out.log
```

</p>
</details>

<details>
<summary>What about M1 macs?</summary>
<p>

M1 macs should work, but not tested.

The challenge is installing Lima on M1 macs, instructions are available on [Lima project page](https://github.com/lima-vm/lima/blob/master/README.md#installation).

</p>
</details>

## License

MIT

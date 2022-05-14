# Colima

[![Go](https://github.com/abiosoft/colima/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/go.yml)
[![Integration](https://github.com/abiosoft/colima/actions/workflows/integration.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/integration.yml)

Container runtimes on macOS (and Linux) with minimal setup.

![Demonstration](colima.gif)

## Features

- Intel and M1 Macs support
- Simple CLI interface
- Docker and Containerd support
- Port Forwarding
- Volume mounts
- Kubernetes

## Getting Started

### Installation

Colima is available on Homebrew, MacPorts, and Nix. Check [here](INSTALL.md) for other installation options.

```
# Homebrew
brew install colima

# MacPorts
sudo port install colima

# Nix
nix-env -iA nixpkgs.colima
```

Or stay on the bleeding edge (only Homebrew)

```
brew install --HEAD colima
```

### Upgrading

If upgrading from v0.3.4 or lower, it is required to start afresh by deleting existing instance.

```sh
colima delete # delete existing instance
colima start
```

## Usage

Start Colima with defaults

```
colima start
```

For more usage options

```
colima --help
colima start --help
```

Or use a config file

```
colima start --edit
```

## Runtimes

On initial startup, Colima initiates with a user specified runtime that defaults to Docker.

### Docker

Docker client is required for Docker runtime. Installable with brew `brew install docker`.

You can use the `docker` client on macOS after `colima start` with no additional setup.

### Containerd

`colima start --runtime containerd` starts and setup Containerd. You can use `colima nerdctl` to interact with
Containerd using [nerdctl](https://github.com/containerd/nerdctl).

It is recommended to run `colima nerdctl install` to install `nerdctl` alias script in $PATH.

### Kubernetes

kubectl is required for Kubernetes. Installable with `brew install kubectl`.

To enable Kubernetes, start Colima with `--kubernetes` flag.

```
colima start --kubernetes
```

#### Interacting with Image Registry

For Docker runtime, images built or pulled with Docker are accessible to Kubernetes.

For Containerd runtime, images built or pulled in the `k8s.io` namespace are accessible to Kubernetes.

### Customizing the VM

The default VM created by Colima has 2 CPUs, 2GiB memory and 60GiB storage.

The VM can be customized either by passing additional flags to `colima start`.
e.g. `--cpu`, `--memory`, `--disk`, `--runtime`.
Or by editing the config file with `colima start --edit`.

**NOTE**: disk size cannot be changed after the VM is created.

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

## Project Status

⚠️ The project is still in active early stage development and updates may introduce breaking changes.

## What is with the name?

Colima means Containers in [Lima](https://github.com/lima-vm/lima).

Since Lima is aka Linux on Mac. By transitivity, Colima can also mean Containers on Linux on Mac.

## FAQ

<details>
<summary>How does Colima compare to Lima?</summary>
<p>

Colima is basically a higher level usage of Lima and utilises Lima to provide Docker, Containerd and/or Kubernetes.

If you want more control over the underlying VM, you can either use Lima directly or override Colima's VM settings with [Lima overrides](https://github.com/lima-vm/lima/blob/873a39c6652fe5fcb07ee08418f39ccaeeea6979/pkg/limayaml/default.yaml#L271).

</p>
</details>

<details>
<summary>Can it run alongside Docker for Mac?</summary>
<p>
Yes, from version v0.3.0 Colima leverages Docker contexts and can thereby run alongside Docker for Mac.

`docker context list` can list all contexts and `docker context use` can be used to change the active context.

</p>
</details>

<details>
<summary>How to customize Docker config e.g. add insecure registries?</summary>
<p>

### v0.3.4 or older

On first startup, Colima generates Docker daemon.json file at `$HOME/.colima/docker/daemon.json`.

Simply modify the daemon.json file accordingly and restart Colima.

### v0.4.0 or newer

Start Colima with `--edit` flag `colima start --edit` and add the config to the `docker` section.

To manually modify the config file, it is located at `$HOME/.colima/default/colima.yaml` for the default profile,
, `$HOME/.colima/<profile>/colima.yaml` for other profiles, and `$HOME/.colima/_templates/default.yaml` for the default
template.

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

### Compatibility

Colima with Docker runtime is compatible with Kind and K3d.

</p>
</details>

<details>
<summary>Are M1 macs supported?</summary>
<p>

Colima supports and works on M1 macs but not rigorously tested as the author do not currently possess an M1 device.
Feedbacks would be appreciated.

</p>
</details>

<details>
<summary>Can I set default configurations?</summary>
<p>

Yes, via the `template` command.

```
colima template
```

Use a preferred editor by setting `$EDITOR` or passing the `--editor` flag

```sh
colima start --edit --editor code # one-off edit
colima template --editor code # set the default config
```

</p>
</details>

## Help Wanted

- Documentation (wiki pages)
- Testing on M1 Macs

## Sponsoring the Project

If you (or your company) are benefiting from the project and would like to support the contributors, kindly support the project on [Patreon](https://patreon.com/colima).

## License

MIT

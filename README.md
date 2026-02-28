![colima-logo](colima.png)

## Colima - container runtimes on macOS (and Linux) with minimal setup.

[![Go](https://github.com/abiosoft/colima/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/go.yml)
[![Integration](https://github.com/abiosoft/colima/actions/workflows/integration.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/integration.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/abiosoft/colima)](https://goreportcard.com/report/github.com/abiosoft/colima)

![Demonstration](colima.gif)

**Website & Documentation:** [colima.run](https://colima.run) | [colima.run/docs](https://colima.run/docs/)

## Features

Support for Intel and Apple Silicon macOS, and Linux

- Simple CLI interface with sensible defaults
- Automatic Port Forwarding
- Volume mounts
- Multiple instances
- Support for multiple container runtimes
  - [Docker](https://docker.com) (with optional Kubernetes)
  - [Containerd](https://containerd.io) (with optional Kubernetes)
  - [Incus](https://linuxcontainers.org/incus) (containers and virtual machines)
- GPU accelerated containers for AI workloads

## Getting Started

### Installation

Colima is available on Homebrew, MacPorts, Nix and [Mise](http://github.com/jdx/mise). Check [here](docs/INSTALL.md) for other installation options.

```sh
# Homebrew
brew install colima

# MacPorts
sudo port install colima

# Nix
nix-env -iA nixpkgs.colima

# Mise
mise use -g colima@latest

```

Or stay on the bleeding edge (only Homebrew)

```
brew install --HEAD colima
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

Docker client is required for Docker runtime. Installable with `brew install docker`.

```
colima start
docker run hello-world
docker ps
```

You can use the `docker` client on macOS after `colima start` with no additional setup.

### Containerd

`colima start --runtime containerd` starts and setup Containerd. You can use `colima nerdctl` to interact with
Containerd using [nerdctl](https://github.com/containerd/nerdctl).

```
colima start --runtime containerd
nerdctl run hello-world
nerdctl ps
```

It is recommended to run `colima nerdctl install` to install `nerdctl` alias script in $PATH.

### Kubernetes

kubectl is required for Kubernetes. Installable with `brew install kubectl`.

To enable Kubernetes, start Colima with `--kubernetes` flag.

```
colima start --kubernetes
kubectl run caddy --image=caddy
kubectl get pods
```

#### Interacting with Image Registry

For Docker runtime, images built or pulled with Docker are accessible to Kubernetes.

For Containerd runtime, images built or pulled in the `k8s.io` namespace are accessible to Kubernetes.

### Incus

<small>**Requires v0.7.0**</small>


Incus client is required for Incus runtime. Installable with brew `brew install incus`.

`colima start --runtime incus` starts and setup Incus.

```
colima start --runtime incus
incus launch images:alpine/edge
incus list
```

You can use the `incus` client on macOS after `colima start` with no additional setup.

**Note:** Running virtual machines on Incus is only supported on m3 or newer Apple Silicon devices.

### AI Models (GPU Accelerated)

<small>**Requires v0.10.0, Apple Silicon and macOS 13+**</small>

Colima supports GPU accelerated containers for AI workloads using the `krunkit` VM type.

**Note:** To use krunkit with colima, ensure it is installed.

```
brew tap slp/krunkit
brew install krunkit
```

Setup and use a model.

```
colima start --runtime docker --vm-type krunkit
colima model run gemma3
```

Colima supports two model runner backends:

- **Docker Model Runner** (default) — supports [Docker AI Registry](https://hub.docker.com/u/ai) and [HuggingFace](https://huggingface.co).
- **Ramalama** — supports [HuggingFace](https://huggingface.co) and [Ollama](https://ollama.com) registries.

The default registry is the Docker AI Registry. Models can be run by name without a prefix:

```sh
colima model run gemma3
colima model run llama3.2
# HuggingFace (Docker Model Runner)
colima model run hf.co/microsoft/Phi-3-mini-4k-instruct-gguf
# Ollama (requires ramalama runner)
colima model run ollama://gemma3 --runner ramalama
```

See the [AI Workloads documentation](https://colima.run/docs/ai/) for more details.

### Customizing the VM

The default VM created by Colima has 2 CPUs, 2GiB memory and 100GiB storage.

The VM can be customized either by passing additional flags to `colima start`.
e.g. `--cpu`, `--memory`, `--disk`, `--runtime`.
Or by editing the config file with `colima start --edit`.

**NOTE**: Disk size can be increased after the VM is created.

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

- create VM with Rosetta 2 emulation. Requires v0.5.3 and macOS >= 13 (Ventura) on Apple Silicon.

  ```
  colima start --vm-type=vz --vz-rosetta
  ```

## Project Goal

To provide container runtimes on macOS with minimal setup.

## What is with the name?

Colima means Containers on [Lima](https://github.com/lima-vm/lima).

Since Lima is aka Linux Machines. By transitivity, Colima can also mean Containers on Linux Machines.

## And the Logo?

The logo was contributed by [Daniel Hodvogner](https://github.com/dhodvogner). Check [this issue](https://github.com/abiosoft/colima/issues/781) for more.

## Troubleshooting and FAQs

Check [here](docs/FAQ.md) for Frequently Asked Questions, or visit the [online FAQ](https://colima.run/docs/faq/) for a searchable version.

## How to Contribute?

Check [here](docs/CONTRIBUTE.md) for the instructions on contributing to the project.

## Community
- [GitHub Discussions](https://github.com/abiosoft/colima/discussions)
- [GitHub Issues](https://github.com/abiosoft/colima/issues)
- [Announcements](https://colima.run/announcements/)
- `#colima` channel in the CNCF Slack
  - New account: <https://slack.cncf.io/>
  - Login: <https://cloud-native.slack.com/>

## License

MIT


## Sponsoring the Project

If you (or your company) are benefiting from the project and would like to support the contributors, kindly sponsor.

- [Github Sponsors](https://github.com/sponsors/abiosoft)
- [Buy me a coffee](https://www.buymeacoffee.com/abiosoft)
- [Patreon](https://patreon.com/colima)

---

[<img src="https://uploads-ssl.webflow.com/5ac3c046c82724970fc60918/5c019d917bba312af7553b49_MacStadium-developerlogo.png" style="max-height: 150px"/>](https://macstadium.com)



![colima-logo](colima.png)

## Colima - container runtimes on macOS (and Linux) with minimal setup.

[![Go](https://github.com/abiosoft/colima/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/go.yml)
[![Integration](https://github.com/abiosoft/colima/actions/workflows/integration.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/integration.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/abiosoft/colima)](https://goreportcard.com/report/github.com/abiosoft/colima)

![Demonstration](colima.gif)

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
  - Apple Container (with optional Kubernetes, macOS 15+)

## Getting Started

### Installation

Colima is available on Homebrew, MacPorts, Nix and [mise](http://github.com/jdx/mise). Check [here](docs/INSTALL.md) for other installation options.

```
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

### Upgrading

If upgrading from v0.5.6 or lower, it is required to start afresh by deleting existing instance.

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

## Using Templates
When you run the `colima template` command, Colima opens the default configuration in a temporary file using your editor (VS Code by default, if installed).

For example, you might see something like:
```sh
/var/folders/hm/xmq4vxs13dl2hx2jyct65r080000gn/T/colima-2758922589.yaml

```
You can edit this temporary file as needed. Once you save and close the file in the editor, Colima automatically overwrites the default template config located at:
```sh
~/.colima/_templates/default.yaml

```
To see more options for working with templates, run:
```
colima template --help

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

### Incus

<small>**Requires v0.7.0**</small>


Incus client is required for Incus runtime. Installable with brew `brew install incus`.

`colima start --runtime incus` starts and setup Incus.

You can use the `incus` client on macOS after `colima start` with no additional setup.

**Note:** Running virtual machines on Incus is only supported on m3 or newer Apple Silicon devices.

### Apple Container

<small>**Requires v0.8.0 and macOS 15+**</small>

Apple Container uses the native containerization framework available in macOS 15 and later.

`colima start --runtime apple` starts and setup Apple Container.

You can use the `containerization` client on macOS after `colima start` with no additional setup.

**Note:** Apple Container is only available on macOS 15 or newer and requires the containerization framework.

### None

<small>**Requires v0.7.0**</small>

Colima can also be utilised solely as a headless virtual machine manager by specifying `none` runtime.


### Customizing the VM

The default VM created by Colima has 2 CPUs, 2GiB memory and 100GiB storage.

The VM can be customized either by passing additional flags to `colima start`.
e.g. `--cpu`, `--memory`, `--disk`, `--runtime`.
Or by editing the config file with `colima start --edit`.

**NOTE**: ~~disk size cannot be changed after the VM is created.~~ From v0.5.3, disk size can be increased.

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

Check [here](docs/FAQ.md) for Frequently Asked Questions.

## How to Contribute?

Check [here](docs/CONTRIBUTE.md) for the instructions on contributing to the project.

## Community
- [GitHub Discussions](https://github.com/abiosoft/colima/discussions)
- [GitHub Issues](https://github.com/abiosoft/colima/issues)
- `#colima` channel in the CNCF Slack
  - New account: <https://slack.cncf.io/>
  - Login: <https://cloud-native.slack.com/>

## Help Wanted

- Documentation and project website

## License

MIT


## Sponsoring the Project

If you (or your company) are benefiting from the project and would like to support the contributors, kindly sponsor.

- [Github Sponsors](https://github.com/sponsors/abiosoft)
- [Buy me a coffee](https://www.buymeacoffee.com/abiosoft)
- [Patreon](https://patreon.com/colima)

---

[<img src="https://uploads-ssl.webflow.com/5ac3c046c82724970fc60918/5c019d917bba312af7553b49_MacStadium-developerlogo.png" style="max-height: 150px"/>](https://macstadium.com)



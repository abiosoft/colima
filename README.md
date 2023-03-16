![colima-logo](colima.png)

## Colima - container runtimes on macOS (and Linux) with minimal setup.

[![Go](https://github.com/abiosoft/colima/actions/workflows/go.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/go.yml)
[![Integration](https://github.com/abiosoft/colima/actions/workflows/integration.yml/badge.svg)](https://github.com/abiosoft/colima/actions/workflows/integration.yml)

![Demonstration](colima.gif)

## Features

- Intel and M1 Macs support
- Simple CLI interface
- Docker and Containerd support
- Port Forwarding
- Volume mounts
- Kubernetes
- Multiple instances

## Getting Started

### Installation

Colima is available on Homebrew, MacPorts, and Nix. Check [here](docs/INSTALL.md) for other installation options.

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

If upgrading from v0.4.6 or lower, it is required to start afresh by deleting existing instance.

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

**NOTE**: ~~disk size cannot be changed after the VM is created.~~ From v0.5.3, disk size can be increased

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

- create VM with Rosetta 2 emulation. Requires v0.5.3 and  MacOS >= 13 (Ventura)

  ```
  colima start --arch aarch64 --vm-type=vz --vz-rosetta
  ```

## Project Goal

To provide container runtimes on macOS with minimal setup.

## What is with the name?

Colima means Containers in [Lima](https://github.com/lima-vm/lima).

Since Lima is aka Linux on Mac. By transitivity, Colima can also mean Containers on Linux on Mac.

## Troubleshooting and FAQs

Check [here](docs/FAQ.md) for Frequently Asked Questions.

## Help Wanted

- Documentation (wiki pages)

## License

MIT


## Sponsoring the Project

If you (or your company) are benefiting from the project and would like to support the contributors, kindly support the project.

<a href="https://www.buymeacoffee.com/abiosoft" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-blue.png" alt="Buy Me A Coffee" style="height: 40px !important;width: 160px !important;" ></a>

[<img src="https://uploads-ssl.webflow.com/5ac3c046c82724970fc60918/5c019d917bba312af7553b49_MacStadium-developerlogo.png" style="max-height: 150px"/>](https://macstadium.com)



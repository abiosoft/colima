# Installation Options

## Homebrew

Stable Version

```
brew install colima
```

Development Version

```
brew install --HEAD colima
```

## MacPorts

Stable version

```
sudo port install colima
```

## Nix

Only stable Version

```
nix-env -i colima
```

Or using solely in a `nix-shell`

```
nix-shell -p colima
```

## Arch

Install dependencies
```
sudo pacman -S qemu-full go docker
```
Install Lima and Colima from Aur
```
yay -S lima-bin colima-bin
```


## Runtime dependencies for manual installs

Before running a downloaded binary or a source build, install [Lima](https://lima-vm.io/docs/installation/) and ensure `lima` and `limactl` are in your `PATH`.
On Linux, install QEMU as described in the Lima installation guide, including `qemu-img` and the system emulator for your VM architecture.

Install the Docker client for the default Docker runtime, or use `colima start --runtime containerd` to use Containerd.
Kubernetes additionally requires `kubectl` on the host.

Confirm Lima is available before starting Colima.

```sh
limactl --version
```

## Binary

Binaries are available with every release on the [releases page](https://github.com/abiosoft/colima/releases).

```sh
# download binary
curl -LO https://github.com/abiosoft/colima/releases/latest/download/colima-$(uname)-$(uname -m)

# install in $PATH
sudo install colima-$(uname)-$(uname -m) /usr/local/bin/colima
```

## Building from Source

Requires [Go](https://golang.org). Install the [runtime dependencies](#runtime-dependencies-for-manual-installs) before starting Colima. `make install` installs the Colima executable.

```sh
# clone repo and cd into it
git clone https://github.com/abiosoft/colima
cd colima
make
sudo make install
```

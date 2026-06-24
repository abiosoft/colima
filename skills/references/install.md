# Installing Colima

Source: `docs/INSTALL.md`. Requires macOS 13+ (or Linux). Docker runtime also needs a docker client
(`brew install docker`).

## Homebrew

```sh
brew install colima          # stable
brew install --HEAD colima   # development version
```

## MacPorts

```sh
sudo port install colima
```

## Nix

```sh
nix-env -i colima            # install (stable only)
nix-shell -p colima          # or use only within a nix-shell
```

## Arch

```sh
sudo pacman -S qemu-full go docker   # dependencies
yay -S lima-bin colima-bin           # Lima + Colima from AUR
```

## Binary

Binaries ship with every [release](https://github.com/abiosoft/colima/releases).

```sh
# download binary
curl -LO https://github.com/abiosoft/colima/releases/latest/download/colima-$(uname)-$(uname -m)

# install in $PATH
sudo install colima-$(uname)-$(uname -m) /usr/local/bin/colima
```

## Building from source

Requires [Go](https://golang.org).

```sh
git clone https://github.com/abiosoft/colima
cd colima
make
sudo make install
```

## Notes

- Colima requires **macOS 13 or newer**. On older macOS you may be able to build Colima and its
  dependencies ([Lima](https://github.com/lima-vm/lima), [Qemu](https://www.qemu.org/)) from source.
- Both Intel and Apple Silicon Macs are supported.
- Updating: `brew upgrade colima`, then `colima delete && colima start` to recreate the VM on the new
  image. The container runtime alone can be updated with `colima update` (v0.7.6+). See
  `troubleshooting.md` → updates.

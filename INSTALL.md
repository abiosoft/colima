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

## Binary

Binaries are available with every release on the [releases page](https://github.com/abiosoft/colima/releases).

```sh
# download binary
curl -LO https://github.com/abiosoft/colima/releases/download/v0.3.4/colima-$(uname)-$(uname -m)

# install in $PATH
install colima-$(uname)-$(uname -m) /usr/local/bin/colima # or sudo install if /usr/local/bin requires root.
```

## Building from Source

Requires [Go](https://golang.org).

```sh
# clone repo and cd into it
git clone https://github.com/abiosoft/colima
cd colima
make
make install # or `sudo make install` if /usr/local/bin requires root
```

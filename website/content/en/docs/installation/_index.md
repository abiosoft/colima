---
title: Installation
weight: 1
---

Supported host OS:

- macOS (the latest version is recommended)
- Linux

Prerequisites:

- Docker Desktop is not required (Colima provides its own Docker runtime)
- QEMU may be required for some configurations

{{< tabpane text=true >}}

{{% tab header="Homebrew" %}}

```bash
brew install colima
```

Hint: specify `--HEAD` to install the HEAD (master) version.
The HEAD version provides early access to the latest features and improvements before they are officially released.

Homebrew formula is available [here](https://github.com/Homebrew/homebrew-core/blob/master/Formula/c/colima.rb).
Supports macOS and Linux.
{{% /tab %}}

{{% tab header="MacPorts" %}}

```bash
sudo port install colima
```

Port: <https://ports.macports.org/port/colima/>
{{% /tab %}}

{{% tab header="Nix" %}}

```bash
nix-env -i colima
```

Nix file: <https://github.com/NixOS/nixpkgs/blob/master/pkgs/by-name/co/colima/package.nix>
{{% /tab %}}

{{% tab header="Binary" %}}
Download the binary archive of Colima from <https://github.com/abiosoft/colima/releases>,
and extract it under `/usr/local/bin` (or somewhere else).

# download binary

curl -LO https://github.com/abiosoft/colima/releases/latest/download/colima-$(uname)-$(uname -m)

# install in $PATH

sudo install colima-$(uname)-$(uname -m) /usr/local/bin/colima

{{% /tab %}}
{{< /tabpane >}}

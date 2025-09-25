---
title: Source Installation
weight: 30
---

## Installing from Source

If you prefer to build Colima from source, follow these steps:

### Prerequisites
Ensure you have the following dependencies installed:
- `git`
- `go`
- `make`

### Build and Install
Run the following commands:

```bash
git clone git@github.com:abiosoft/colima.git
cd colima
make build
sudo mv ./_output/binaries/colima-Darwin-arm64 /opt/homebrew/bin/colima
```

> **Note:** `brew install --head colima`  installs from the `main` branch.

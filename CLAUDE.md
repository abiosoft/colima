# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
make build                  # builds to _output/binaries/colima-<OS>-<ARCH>
make install                # installs to /usr/local/bin/colima

# Test
make test                   # go test -v ./...
go test ./path/to/pkg/...   # run a single package's tests
go test -run TestName ./... # run a specific test

# Lint/Format
make lint                   # runs golangci-lint (must be installed separately)
make fmt                    # go fmt + goimports

# Other
make vmnet                  # builds the vmnet helper binary (macOS network)
make integration            # builds then runs integration tests
```

## Architecture

Colima is a CLI tool that wraps [Lima](https://github.com/lima-vm/lima) to run container runtimes (Docker, containerd, Incus) and Kubernetes inside a Linux VM on macOS (and Linux).

### Layered structure

```
cmd/          → Cobra CLI commands (start, stop, delete, ssh, status, …)
app/          → App interface: orchestrates VM + container runtime lifecycle
environment/  → Core interfaces (VM, Container, HostActions, GuestActions)
  vm/lima/    → Lima-backed VM implementation (only VM backend)
  container/  → Container runtime implementations: docker, containerd, kubernetes, incus, podman
  host/       → Host-side actions (runs commands on macOS)
config/       → Config struct, Profile management, file path helpers
daemon/       → Host-side background processes (vmnet for networking, inotify for mounts)
cli/          → CommandChain pattern for sequencing/staging operations
store/        → Persistent JSON state (disk formatted flag, runtime, etc.)
core/         → binfmt/QEMU cross-arch emulation setup
model/        → AI model runner support (ramalama, docker model runner)
embedded/     → Embedded assets (sudoers, disk images SHA list)
```

### Key design patterns

**CommandChain** (`cli/chain.go`): Operations are built as a chain of `Add(func() error)` steps, executed sequentially via `a.Exec()`. `a.Stage("label")` sets a human-readable status. `a.Retry(...)` wraps a step with retries. Errors abort the chain.

**Container runtime registry**: Runtimes self-register via `environment.RegisterContainer(name, func, hidden)` in their `init()` functions. The `environment.NewContainer()` factory dispatches to the registered constructor. Adding a new runtime means implementing `environment.Container` and calling `RegisterContainer` at init time.

**Profile system**: Multiple Colima instances are supported via profiles. The active profile is set by `config.SetProfile()` (driven by `--profile` flag or `COLIMA_PROFILE` env var). The default profile ID is `"colima"`. Profile determines all file paths (Lima config, state file, SSH config, store).

**Start/Stop ordering**: Start: daemon start → VM start → after-boot provision scripts → container runtime provision → container runtime start → ready provision scripts. Stop is the reverse: container runtimes (reverse order) → VM stop.

**VM type selection**: Defaults to `vz` (macOS Virtualization.Framework) on macOS 13+, `qemu` otherwise. `krunkit` is also supported on Apple Silicon.

### Important files

| File | Purpose |
|------|---------|
| `app/app.go` | `App` interface + `colimaApp` implementation — the main orchestrator |
| `environment/environment.go` | `HostActions`, `GuestActions`, `runActions`, `fileActions` interfaces |
| `environment/vm.go` | `VM` interface, `Arch` type, `DefaultVMType()` |
| `environment/container.go` | `Container` interface, runtime registry |
| `environment/vm/lima/lima.go` | Lima VM: start, stop, restart, SSH |
| `environment/vm/lima/limaconfig/config.go` | Lima YAML config generation |
| `config/config.go` | `Config` struct (all user-settable fields) |
| `config/profile.go` | Profile paths and ID logic |
| `config/files.go` | All XDG-style directory helpers (`~/.colima/`, `~/.lima/`) |
| `daemon/process/vmnet/vmnet.go` | vmnet daemon for VM networking on macOS |
| `daemon/process/inotify/` | inotify-based file sync for mounts |

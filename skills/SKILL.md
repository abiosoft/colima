---
name: colima
description: >-
  Guide to using Colima — container runtimes (Docker, containerd, Kubernetes, Incus) on macOS
  and Linux via lightweight Lima VMs. Use this skill whenever Colima is (or should be) the
  container backend: installing Colima; `colima start/stop/status/delete/ssh`; picking or
  switching a runtime; the `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`
  error; Docker contexts and socket location; registry mirrors / insecure registries; buildx;
  bind or volume mounts that show up empty in the container; disk space recovery/resize;
  reachable VM IP; GPU / AI model workloads; config files, profiles and `COLIMA_HOME`; or a
  Colima VM that won't start. ALSO use it for **writing scripts that drive Colima** — bootstrap,
  dev-env, deploy, or CI scripts that bring Colima up non-interactively — because the skill has
  the correct flags, profile-specific socket paths, idempotent `colima start` guards, and
  readiness/teardown patterns that hand-written scripts routinely get wrong (e.g. inventing a
  non-existent flag or hardcoding the wrong docker socket). Prefer this over generic Docker
  advice whenever Colima is the daemon. This skill is specifically about **Colima** — not Docker
  Desktop, minikube, Kind, K3d, Rancher Desktop, OrbStack, Podman, or plain `limactl`.
---

# Colima

Colima runs **container runtimes on macOS (and Linux) with minimal setup**. It's a higher-level
wrapper over [Lima](https://github.com/lima-vm/lima): it boots a small VM and provisions Docker,
containerd, Kubernetes and/or Incus inside it, then wires up the host client so `docker` (etc.)
"just works".

> **Scope:** the canonical source is the repo's own docs (`docs/INSTALL.md`, `docs/FAQ.md`) plus
> README usage. Behavior is **version-gated** in many places — keep the `vX.Y.Z` notes when
> advising, since users run a range of versions. Check the user's version with `colima version`.

## Quick start

```sh
brew install colima      # + docker client if using Docker runtime: brew install docker
colima start             # boots the VM, Docker runtime by default
docker run hello-world   # docker client works with no extra setup
```

Full install options (MacPorts, Nix, Arch, binary, source) → `references/install.md`.

## Core commands

| Command | What it does |
|---|---|
| `colima start [profile]` | Create/start the VM. Flags: `--runtime`, `--kubernetes`, `--cpu`, `--memory`, `--disk`, `--edit`, `--env`, `--dns`, `--network-address`, `--vm-type`, `--foreground` |
| `colima stop [--force]` | Stop the VM. `--force` recovers a `Broken` status |
| `colima status` | Show running status (incl. Docker socket path) |
| `colima list` | List profiles and their status/arch/cpus/mem/disk/runtime |
| `colima restart` | Stop + start (needed after editing mounts/config) |
| `colima delete [--data]` | Delete the VM. `--data` also deletes container data (since v0.9.0 data lives on a separate disk) |
| `colima ssh [-- <cmd>]` | Shell into the VM, or run one command |
| `colima start --edit` | Start while editing the YAML config (since v0.4.0) |
| `colima template` | Edit the default config template for new instances |
| `colima update` | Update the container runtime in place (since v0.7.6) |
| `colima nerdctl ...` | nerdctl wrapper for the containerd runtime |
| `colima model ...` | Set up / run AI models (GPU, krunkit; since v0.10.0) |
| `colima version` | Print the version (use it before applying version-gated advice) |

Profiles isolate independent VMs: `colima start work` operates on the `work` profile; default is `default`.
`colima --help` / `colima start --help` list everything.

## Runtimes — chosen at `start`

Default is Docker. **Switching the runtime requires re-creating the VM** — a stop/start alone
does not change it: `colima delete --data && colima start --runtime <new>`.

- **Docker** (default): `colima start` → `docker ...` works directly. Needs the docker client (`brew install docker`).
- **Containerd**: `colima start --runtime containerd` → use `colima nerdctl ...` (run `colima nerdctl install` to add a `nerdctl` alias to `$PATH`).
- **Kubernetes**: `colima start --kubernetes` (needs `kubectl`). Shares images with the chosen container runtime.
- **Incus** (v0.7.0+): `colima start --runtime incus` → `incus ...`.
- **AI models / GPU** (v0.10.0+, Apple Silicon, macOS 13+): `colima start --runtime docker --vm-type krunkit` then `colima model run gemma3`.

Per-runtime config, socket location, registry mirrors, buildx, nerdctl, AI model registries → `references/runtimes.md`.

> **Registry mirror** (common task): `colima start --registry-mirror <url>` (repeatable) or `--edit` →
> add under `docker.registry-mirrors`. For the **client** to honor mirrors, the host
> `~/.docker/daemon.json` may also need the same entries. Details → `references/runtimes.md`.

## Customizing the VM

Defaults: **2 CPUs, 2 GiB memory, 100 GiB disk**. Customize via flags or the config file.

```sh
colima start --cpu 4 --memory 8 --disk 100      # at create time
colima stop && colima start --cpu 4 --memory 8  # change an existing VM (disk can only grow)
colima start --vm-type=vz --vz-rosetta          # Rosetta 2 (v0.5.3+, Apple Silicon, macOS 13+)
```

Config files, profiles, `COLIMA_HOME`, passing env into the VM, Lima overrides → `references/configuration.md`.

**Writing a bootstrap / CI / deploy script?** See `references/automation.md` for non-interactive patterns:
idempotent start (`colima status` guard), readiness wait (`docker info` loop), `DOCKER_HOST` wiring so the
rest of the script targets Colima, `--foreground` for supervisors, teardown, and a GitHub Actions sketch.

## Common gotchas (quick)

- **`Cannot connect to the Docker daemon at unix:///var/run/docker.sock`** — an app isn't using Colima's
  Docker context. Point it at Colima's socket (`export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"`)
  or relink the socket. Details + socket location by version → `references/troubleshooting.md`.
- **`Broken` status** (e.g. after a macOS restart): `colima stop --force`, then `colima start`.
- **Docker bind mount shows empty** — the host path must be under `/Users/$USER`, or add it to the `mounts`
  section of the config and `colima restart`. Paths with spaces are unsupported.
- **Won't start / fatal error** — re-run with `--verbose`; common causes are no virtualization support or an
  x86_64 Homebrew/Colima on Apple Silicon. See `references/troubleshooting.md`.

Full troubleshooting (no internet/DNS, runc/cgroups error, disk recovery & resize, reachable IP, post-upgrade
issues, deleting data) → `references/troubleshooting.md`.

## References

- `references/install.md` — all install methods (Homebrew, MacPorts, Nix, Arch, binary, source).
- `references/configuration.md` — config file & profiles, file locations & env vars, custom VM env, Lima overrides, autostart.
- `references/runtimes.md` — Docker (socket/contexts/registries/buildx), Containerd, Kubernetes, Incus, AI models; comparisons to Lima/minikube/Kind/K3d.
- `references/troubleshooting.md` — startup failures, networking, mounts, disk, updates, deleting data.
- `references/automation.md` — non-interactive patterns for bootstrap / CI / deploy scripts.

# Runtimes: Docker, Containerd, Kubernetes, Incus, AI

Sources: `README.md` (usage) + `docs/FAQ.md` (config/details). Runtime is chosen at `colima start`
(default Docker). **Switching to a different runtime requires re-creating the VM** — a stop/start
alone does not change it:

```sh
colima delete --data && colima start --runtime <new runtime>
```

## Docker

```sh
colima start            # docker runtime
docker run hello-world
docker ps
```

Colima leverages **Docker contexts** and can run alongside Docker Desktop (since v0.3.0); it makes
itself the default context on startup.

### Socket location (version-gated)

| Version | Docker socket |
|---|---|
| v0.3.4 or older | `$HOME/.colima/docker.sock` |
| v0.4.0 or newer | `$HOME/.colima/default/docker.sock` |

Also shown by `colima status`.

### Docker contexts

```sh
docker context list           # list contexts
docker context use <name>     # switch active context
```

### Customizing the Docker daemon (insecure registries, mirrors)

- v0.3.4 or lower: edit `$HOME/.colima/docker/daemon.json` (generated on first start), then restart.
- v0.4.0+: `colima start --edit` and add under the `docker:` section:

  ```diff
  - docker: {}
  + docker:
  +   insecure-registries:
  +     - myregistry.com:5000
  +   registry-mirrors:
  +     - https://my.dockerhub.mirror.something
  ```

Registry mirrors can also be set at start (repeatable flag), no editing needed:

```sh
colima start --registry-mirror https://my.dockerhub.mirror.something
```

> The host `~/.docker/daemon.json` may also need matching changes for the **client** to honor some
> values (e.g. registry mirrors). Use `colima template` to bake changes into all new instances.

### Buildx plugin missing

```sh
brew install docker-buildx
mkdir -p ~/.docker/cli-plugins
ln -sfn $(which docker-buildx) ~/.docker/cli-plugins/docker-buildx
docker buildx version
```

## Containerd

```sh
colima start --runtime containerd
colima nerdctl install      # add a `nerdctl` alias to $PATH (recommended)
nerdctl run hello-world
nerdctl ps
```

### Config files (first start with containerd)

| File | Location |
|---|---|
| Containerd config | `~/.config/containerd/config.toml` |
| BuildKit config | `~/.config/buildkit/buildkitd.toml` |

Shared across profiles; edit then `colima stop && colima start --runtime containerd`.
`$XDG_CONFIG_HOME` is respected if set.

**Per-profile override:** place the file at `$HOME/.colima/<profile>/containerd/config.toml` (or
`buildkitd.toml`). Resolution order: per-profile override → central (`~/.config/...`) → embedded default.

## Kubernetes

```sh
colima start --kubernetes    # needs kubectl (brew install kubectl)
kubectl run caddy --image=caddy
kubectl get pods
```

Image registry sharing: Docker runtime — images built/pulled with Docker are available to Kubernetes;
Containerd runtime — images in the `k8s.io` namespace are available.

## Incus (v0.7.0+)

```sh
colima start --runtime incus    # needs incus (brew install incus)
incus launch images:alpine/edge
incus list
```

Running **VMs** on Incus needs an M3 or newer Apple Silicon device. Incus instances aren't reachable
from the host by default (v0.10.0+) — enable network address:

```sh
colima stop
colima start --network-address
```

## AI models / GPU (v0.10.0+, Apple Silicon, macOS 13+)

Uses the `krunkit` VM type (install [krunkit](https://github.com/containers/krunkit#installation) first).

```sh
colima start --runtime docker --vm-type krunkit
colima model setup
colima model run gemma3
```

Two runner backends: **Docker Model Runner** (default — Docker AI Registry + HuggingFace) and
**Ramalama** (HuggingFace + Ollama). Registry examples:

```sh
colima model run gemma3                      # Docker AI Registry (default, no prefix)
colima model run hf://tinyllama              # HuggingFace
colima model run ollama://tinyllama --runner ramalama   # Ollama (ramalama runner)
```

`colima model --help` for more.

## How Colima compares

- **vs Lima:** Colima is a higher-level use of Lima — it uses Lima to provide Docker/Containerd/Kubernetes.
- **vs minikube/Kind/K3d (Kubernetes):** those suit multiple clusters or when you don't need Docker and
  Kubernetes to share images/runtime. Colima with Docker runtime is fully compatible with them.
- **vs minikube (Docker):** minikube can expose its Docker via `minikube docker-env`, but Kubernetes is
  not optional there, and its free macOS drivers fall short on performance/port-forwarding/volumes for
  Docker-only use.

## Distro / underlying VM

- v0.5.6 and lower: lightweight Alpine image; optional Ubuntu **layer** via `colima start --layer=true`
  (with the layer, `ssh`/`ssh-config` route to the layer; use `--layer=false` to reach the underlying VM).
- v0.6.0 and newer: Ubuntu is the underlying image; other distros aren't supported.

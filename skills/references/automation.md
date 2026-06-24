# Automation: bootstrap & deployment scripts

Patterns for driving Colima **non-interactively** — dev-env bootstrap, CI, deploy scripts. The repo
docs are written for interactive use; this page adapts the same commands into script-safe building
blocks. Pin behavior to a version (`colima version`) since flags vary.

## 1. Wire the rest of the script to Colima's Docker

A script that runs `docker`/`docker compose` after starting Colima must target Colima's daemon. Two ways:

```sh
# A) point this shell at Colima's socket (v0.4.0+ path)
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"

# B) or make Colima the active context (it already sets itself default on start)
docker context use colima
```

Don't hardcode the socket blindly — `colima start` already makes itself the default context, so often no
export is needed. Use the explicit `DOCKER_HOST` when the script runs tools that ignore Docker contexts
(see `troubleshooting.md`). Confirm the path with `colima status`.

## 2. Idempotent start

Re-running the script shouldn't fail if Colima is already up. `colima status` exits non-zero when the
profile isn't running:

```sh
#!/usr/bin/env bash
set -euo pipefail

PROFILE="${COLIMA_PROFILE:-default}"

if ! colima status --profile "$PROFILE" >/dev/null 2>&1; then
  colima start --profile "$PROFILE" \
    --cpu 4 --memory 8 --disk 100 \
    --runtime docker
else
  echo "colima '$PROFILE' already running"
fi
```

Use a dedicated `--profile` to isolate the script's VM from a developer's `default` one (and to allow
parallel CI jobs).

## 3. Wait for readiness

`colima start` returns when the VM is provisioned, but in CI it's worth confirming the daemon actually
answers before deploying:

```sh
for i in $(seq 1 30); do
  if docker info >/dev/null 2>&1; then break; fi
  echo "waiting for docker... ($i)"; sleep 2
done
docker info >/dev/null   # final check; fails the script if still not ready
```

## 4. Non-interactive / CI notes

- **No prompts:** lifecycle commands are non-interactive except destructive ones — use `colima delete --force`.
- **Foreground (v0.5.6+):** `colima start --foreground` keeps Colima in the foreground; useful under a
  process supervisor (launchd/systemd) or a CI step that owns the lifecycle. Without it, `colima start`
  backgrounds the VM and returns.
- **Config over flags:** for reproducible environments, commit a `colima.yaml` and apply it with
  `colima start --edit` locally, or bake defaults via `colima template`. See `configuration.md`.
- **Env into the VM:** `colima start --env KEY=value` (repeatable) or the `env:` config block.

## 5. Teardown

```sh
colima stop --profile "$PROFILE"            # stop, keep the VM
colima delete --force --profile "$PROFILE"  # remove the VM (data on separate disk since v0.9.0 is kept)
colima delete --force --data --profile "$PROFILE"  # also wipe container data
```

## 6. Bootstrap script skeleton

```sh
#!/usr/bin/env bash
# Bring up Colima (docker + kubernetes), make docker target it, deploy.
set -euo pipefail
PROFILE="${COLIMA_PROFILE:-ci}"

if ! colima status --profile "$PROFILE" >/dev/null 2>&1; then
  colima start --profile "$PROFILE" --kubernetes --cpu 4 --memory 8 --disk 60
fi

export DOCKER_HOST="unix://$HOME/.colima/$PROFILE/docker.sock"   # profile-specific socket
for i in $(seq 1 30); do docker info >/dev/null 2>&1 && break; sleep 2; done

docker compose up -d            # or: kubectl apply -f k8s/
# ... deploy / test ...

# teardown (e.g. CI trap):
# trap 'colima delete --force --profile "$PROFILE"' EXIT
```

> Note the **profile-specific socket** path `$HOME/.colima/<profile>/docker.sock` — only the `default`
> profile lives at `.../default/docker.sock`.

## 7. macOS CI (GitHub Actions sketch)

Colima needs a macOS runner (nested virtualization). Rough shape:

```yaml
jobs:
  test:
    runs-on: macos-14
    steps:
      - uses: actions/checkout@v4
      - run: brew install colima docker
      - run: colima start --cpu 3 --memory 6 --disk 30
      - run: docker info && docker compose up -d
      # colima is the default context, so docker just works
```

Pin Colima/runner versions for reproducibility; resource flags must fit the runner's limits.

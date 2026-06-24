# Configuration, profiles & file locations

Source: `docs/FAQ.md`.

## Config file (since v0.4.0)

Colima reads a YAML config instead of repeating CLI flags.

```sh
colima start --edit        # edit + start (one-off)
```

- Default config: `$HOME/.colima/default/colima.yaml`
- Other profiles: `$HOME/.colima/<profile-name>/colima.yaml`
- Config location root is `$COLIMA_HOME` (defaults to `$HOME/.colima`).

### Default template for new instances

```sh
colima template                    # edit the default template
```

Template file: `$HOME/.colima/_templates/default.yaml`.

### Choosing the editor

Set `$EDITOR`, or pass `--editor`:

```sh
colima start --edit --editor code   # one-off
colima template --editor code       # default config
```

## File-location environment variables (set on the host)

| Variable | Description |
|---|---|
| `COLIMA_HOME` | Colima config directory (default `$HOME/.colima`) |
| `COLIMA_CACHE_HOME` | Cache directory (host-specific default, see Go `os.UserCacheDir()`) |
| `COLIMA_PROFILE` | Active profile name (default `default`) |
| `DOCKER_CONFIG` | Docker client config dir (default `~/.docker`) |

## Passing custom env vars into the VM

Config file:

```yaml
env:
  MY_VAR: value
```

Or CLI:

```sh
colima start --env MY_VAR=value
# verify inside the VM:
colima ssh -- sh -c 'env | grep MY_VAR'
```

## Autostart

- Foreground mode (since v0.5.6): `colima start --foreground`.
- Installed via brew: `brew services start colima`.

## Lima overrides (advanced users only)

Lima supports `override.yaml` (applied **before** the instance config) and `default.yaml` (applied
**after**, as fallback defaults).

Override file: `$HOME/.colima/_lima/_config/override.yaml` (or `$LIMA_HOME/_config/override.yaml` if
`LIMA_HOME` is set).

> Overriding the **image** is not supported — Colima's image bundles dependencies a custom image would lack.

### Example: provision scripts (run during VM boot)

```yaml
# $HOME/.colima/_lima/_config/override.yaml
provision:
  - mode: system
    script: |
      #!/bin/bash
      set -eux -o pipefail
      apt-get update && apt-get install -y curl
```

Or directly in `colima.yaml` via `colima start --edit`:

```diff
- provision: []
+ provision:
+   - mode: system
+     script: |
+       #!/bin/bash
+       set -eux -o pipefail
+       apt-get update && apt-get install -y curl
```

## Reachable VM IP (macOS only)

Disabled by default (needs root, slower startup). Enable per-start or in config:

```sh
colima start --network-address
```

```diff
network:
-  address: false
+  address: true
```

(For Incus reachability from the host this is required — see `runtimes.md`.)

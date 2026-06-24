# Troubleshooting

Source: `docs/FAQ.md`. Many fixes are version-gated — check `colima version` first.

## `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`

Colima uses Docker contexts and sets itself as the default context on start, but some apps ignore
contexts. Fix with any of (confirm the socket path first — see socket location in `runtimes.md`):

1. Set an app-specific Docker socket path if supported (e.g. JetBrains IDEs).
2. Point `DOCKER_HOST` at Colima's socket:
   ```sh
   export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
   ```
3. Link Colima's socket to the default path (**may break other Docker servers**):
   ```sh
   sudo ln -sf $HOME/.colima/default/docker.sock /var/run/docker.sock
   ```

## Colima not starting

### Broken status

`colima list` shows `Broken` (often after a macOS restart). Force-stop, then start:

```sh
colima stop --force
colima start
```

### `FATA[0000] error starting vm: error at 'starting': exit status 1`

Enable debug logging to investigate:

```sh
colima start --verbose
```

If the log shows `exiting, status={Running:false Degraded:false Exiting:true ...}`, it's almost always
either (1) a device without virtualization support, or (2) an x86_64 Homebrew/Colima on an Apple Silicon (M1) device.

### Issues after an upgrade

Test with a separate profile; if it starts cleanly, reset the default profile:

```sh
colima start debug      # separate profile
# if that works:
colima delete
colima start
```

## Colima cannot access the internet

Usually DNS. Try custom DNS servers and verify from inside the VM:

```sh
colima start --dns 8.8.8.8 --dns 1.1.1.1
colima ssh -- ping -c4 google.com
```

## Docker Compose / Buildx `runc ... cgroup ... invalid argument` (v0.5.6 or lower)

Workaround from v0.5.6: start with `--cgroups-v2`. **Fixed in v0.6.0.**

```sh
colima start --cgroups-v2
```

## Docker bind mount shows empty

Bind-mounting a host path **not** under `/Users/$USER` starts the container without error but the
mountpoint is empty. Mount it on the VM first: edit `$HOME/.colima/default/colima.yaml`, add the path
to the `mounts` section (examples are in the file), then `colima restart` and re-run the container.

## Mount path with spaces is not supported

Paths with spaces (e.g. `/Volumes/External HD`) aren't supported by the underlying Lima runtime; Colima
rejects them at startup with a clear error. Workaround: mount a parent dir without spaces, or rename/alias
the volume. See [#1471](https://github.com/abiosoft/colima/issues/1471).

## Disk space

### Recover space

Free space in the VM with `docker system prune` or by removing containers. On v0.4.x or lower this
doesn't reflect on the host.

- v0.5.0+: unused VM disk space is released on startup — a `colima restart` suffices.
- v0.5.0+: manual trim:
  ```sh
  colima ssh -- sudo fstrim -a      # add -v for verbose
  ```

### Increase disk size (v0.5.3+)

Disk grows on startup from `colima.yaml`:

```diff
- disk: 150
+ disk: 250
```

## Updating

- **Colima itself:** `brew upgrade colima`, then recreate the VM to use the new image:
  ```sh
  colima delete
  colima start
  ```
  Test an upgrade safely on a separate profile: `colima start debug`.
- **Container runtime only (v0.7.6+):** `colima update` — updates Docker/containerd without updating
  Colima or waiting for a release.

## Accessing the VM directly

```sh
colima ssh                 # interactive shell
colima ssh -- uname -a     # one-off command
```

## Deleting container data

Since v0.9.0 container data lives on a separate disk, so `colima delete` preserves it (reinstated on
`colima start`). To wipe everything including data:

```sh
colima delete --data
```

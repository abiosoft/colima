# Physical Disks

Colima can attach macOS physical disk partitions to the VM and mount them as Linux filesystems. The mounted filesystem can also be exposed back to macOS through NFS, so host tools and containers can use the same files.

This is intended for Linux filesystems such as `ext4`, `xfs`, or `btrfs` that macOS does not mount natively.

A physical partition is useful for source trees and other working data that should outlive the Colima VM. The VM disk can be deleted, recreated, or corrupted during troubleshooting; keeping code on a separate host partition lets the VM stay disposable while new changes remain outside the VM image. This is not a replacement for backups, but it reduces the blast radius of VM lifecycle mistakes.

> **Warning**
>
> Writable physical disks are dangerous. Do not mount or write to the same filesystem from another OS while Colima is using it. Colima refuses to start a writable disk when macOS reports that the partition is mounted, but it cannot detect every external writer.

## Example

```yaml
physicalDisks:
  - name: src
    device: /dev/disk0s6
    rawDevice: /dev/rdisk0s6
    fsType: ext4
    writable: true
    mountPoint: /mnt/colima/physical/src
    backend: nbd
    hostAccess:
      enabled: true
      driver: nfs
      mountPoint: /Volumes/Colima/src
```

For native Virtualization.Framework block attachment, use `vmType: vz` and `backend: vz`:

```yaml
vmType: vz
physicalDisks:
  - name: src
    device: /dev/disk0s6
    fsType: ext4
    writable: true
    mountPoint: /mnt/colima/physical/src
    backend: vz
    hostAccess:
      enabled: true
      driver: nfs
      mountPoint: /Volumes/Colima/src
```

Start or restart Colima after editing the profile config:

```sh
colima start --edit
colima restart
```

The disk is mounted in the VM at `mountPoint`. When `hostAccess.enabled` is true, Colima also mounts the VM export on macOS at `hostAccess.mountPoint`.

Colima uses `sudo` on macOS only for operations that require host root privileges. With `backend: nbd`, Colima uses host `sudo` to open the raw partition through `qemu-nbd`, mount the NFS export on macOS, and clean those resources up. With `backend: vz`, Colima does not open the host disk itself; Lima's VZ block-device helper handles the privileged disk open during VM startup, and Colima still uses host `sudo` only for the optional macOS NFS mount. Guest setup uses `sudo` inside the VM to mount the filesystem and manage the optional NFS export.

## Configuration

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Stable disk name. Use letters, numbers, `.`, `_`, or `-`. |
| `device` | yes | macOS partition device, for example `/dev/disk0s6`. Whole disks are rejected. |
| `rawDevice` | no | Raw device path. Defaults from `device`, for example `/dev/rdisk0s6`. |
| `fsType` | no | `auto`, `ext4`, `xfs`, or `btrfs`. Defaults to `auto`. |
| `writable` | no | Mount read-write when true. Defaults to read-only. |
| `mountPoint` | no | VM mount point. Defaults to `/mnt/colima/physical/<name>`. |
| `backend` | no | `auto`, `nbd`, or `vz`. Defaults to `auto`, currently resolved to `nbd`. |
| `hostAccess.enabled` | no | Mount the VM filesystem back on macOS. |
| `hostAccess.driver` | no | `nfs`. |
| `hostAccess.mountPoint` | no | macOS mount point. Defaults to `/Volumes/Colima/<name>`. |

When `hostAccess.enabled` is true, keep `mountPoint` under `/mnt/colima/physical`. Colima exports `/mnt/colima/physical` as the NFSv4 pseudo-root and mounts the disk on macOS as `localhost:/<name>`.

## Backends

### NBD

Colima uses `qemu-nbd` on macOS to expose the raw partition to the VM. The NBD service is bound to `127.0.0.1` and is reachable from the VM through an SSH reverse tunnel, not through the LAN.

Inside the VM, Colima connects the export to `/dev/nbdN`, verifies the filesystem type with `blkid`, and mounts it at `mountPoint`.

### VZ

With `backend: vz`, Colima writes the partition path to Lima's `blockDevices` YAML field. Lima attaches the host partition to the VM through macOS Virtualization.Framework before the guest boots. Inside Linux, Colima mounts the stable virtio device path `/dev/disk/by-id/virtio-<device>`, for example `/dev/disk/by-id/virtio-disk0s6`.

This backend requires:

- `vmType: vz`
- macOS 14 or newer
- a Lima build with secure VZ block-device support, including `limactl sudoers --block-device`
- an installed Lima sudoers file generated with the explicit block-device opt-in

Example Lima sudoers setup:

```sh
limactl sudoers --block-device >etc_sudoers.d_lima
less etc_sudoers.d_lima
sudo install -o root etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

`backend: vz` is faster for VM-side workloads because the guest sees a native virtio block device instead of a network block device. Host access from macOS still uses NFS because macOS does not mount Linux filesystems such as ext4 natively.

For read-only configurations, Colima mounts the filesystem read-only in the guest. The current Lima VZ block-device contract does not provide a host-enforced read-only flag, so use `backend: nbd` if host-enforced read-only block access is required.

## E2E Smoke Test

Contributors can run the native VZ physical disk smoke test on macOS with a Lima build that supports secure block devices:

```sh
COLIMA_BINARY=/path/to/colima \
LIMA_BIN=/path/to/secure-lima/limactl \
scripts/physical_disk_vz_e2e.sh
```

The script creates a temporary ext4 disk image, attaches it as a macOS disk image, installs a temporary root-owned Lima helper under `/opt/colima-vz-e2e`, starts a disposable Colima profile with `backend: vz`, verifies guest and host read/write through the NFS host-access path, and removes the VM, disk image, helper, and sudoers fragment during cleanup.

## Host Access

When host access is enabled, Colima starts an NFS server in the VM and mounts it on macOS through an SSH local tunnel. macOS sees an NFS mount; the Linux VM remains the only machine mounting the physical filesystem directly.

## Troubleshooting

Check the partition identity on macOS:

```sh
diskutil info /dev/disk0s6
```

Check the mounted filesystem in the VM:

```sh
colima ssh -- findmnt /mnt/colima/physical/src
colima ssh -- blkid /dev/nbd0 # backend: nbd
colima ssh -- blkid /dev/disk/by-id/virtio-disk0s6 # backend: vz
```

If Colima reports that a writable disk is mounted on macOS, unmount it first:

```sh
diskutil unmount /dev/disk0s6
```

If host access remains mounted after an interrupted shutdown:

```sh
sudo umount /Volumes/Colima/src
colima stop --force
```

## Limitations

- Physical disks are currently macOS-only.
- `backend: vz` requires `vmType: vz` and a Lima build with secure VZ block-device support.
- Writable mode requires the filesystem to be mounted by Colima only.
- Host access requires the macOS NFS client and the VM NFS server.

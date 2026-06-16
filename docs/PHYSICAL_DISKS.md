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

Start or restart Colima after editing the profile config:

```sh
colima start --edit
colima restart
```

The disk is mounted in the VM at `mountPoint`. When `hostAccess.enabled` is true, Colima also mounts the VM export on macOS at `hostAccess.mountPoint`.

Colima uses `sudo` on macOS only for operations that require host root privileges: opening the raw partition through `qemu-nbd`, mounting the NFS export on macOS, and cleaning those resources up. Guest setup also uses `sudo` inside the VM to connect `/dev/nbdN`, mount the filesystem, and manage the NFS export.

## Configuration

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Stable disk name. Use letters, numbers, `.`, `_`, or `-`. |
| `device` | yes | macOS partition device, for example `/dev/disk0s6`. Whole disks are rejected. |
| `rawDevice` | no | Raw device path. Defaults from `device`, for example `/dev/rdisk0s6`. |
| `fsType` | no | `auto`, `ext4`, `xfs`, or `btrfs`. Defaults to `auto`. |
| `writable` | no | Mount read-write when true. Defaults to read-only. |
| `mountPoint` | no | VM mount point. Defaults to `/mnt/colima/physical/<name>`. |
| `backend` | no | `auto` or `nbd`. Defaults to `auto`, currently resolved to `nbd`. |
| `hostAccess.enabled` | no | Mount the VM filesystem back on macOS. |
| `hostAccess.driver` | no | `nfs`. |
| `hostAccess.mountPoint` | no | macOS mount point. Defaults to `/Volumes/Colima/<name>`. |

When `hostAccess.enabled` is true, keep `mountPoint` under `/mnt/colima/physical`. Colima exports `/mnt/colima/physical` as the NFSv4 pseudo-root and mounts the disk on macOS as `localhost:/<name>`.

## How It Works

Colima uses `qemu-nbd` on macOS to expose the raw partition to the VM. The NBD service is bound to `127.0.0.1` and is reachable from the VM through an SSH reverse tunnel, not through the LAN.

Inside the VM, Colima connects the export to `/dev/nbdN`, verifies the filesystem type with `blkid`, and mounts it at `mountPoint`.

When host access is enabled, Colima starts an NFS server in the VM and mounts it on macOS through an SSH local tunnel. macOS sees an NFS mount; the Linux VM remains the only machine mounting the physical filesystem directly.

## Troubleshooting

Check the partition identity on macOS:

```sh
diskutil info /dev/disk0s6
```

Check the mounted filesystem in the VM:

```sh
colima ssh -- findmnt /mnt/colima/physical/src
colima ssh -- blkid /dev/nbd0
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
- The implemented backend is NBD. Native VZ block device attachment can be added later without changing the `physicalDisks` user-facing schema.
- Writable mode requires the filesystem to be mounted by Colima only.
- Host access requires the macOS NFS client and the VM NFS server.

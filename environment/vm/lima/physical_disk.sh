#!/usr/bin/env sh
set -eu

NAME="$1"
NBD_DEVICE="$2"
NBD_PORT="$3"
MOUNT_POINT="$4"
FS_TYPE="$5"
WRITABLE="$6"

EXPORT_FILE="/etc/exports.d/colima-physical-${NAME}.exports"
EXPORT_ROOT="/mnt/colima/physical"
EXPORT_ROOT_FILE="/etc/exports.d/colima-physical-root.exports"

install_packages() {
	if command -v nbd-client >/dev/null 2>&1 && command -v exportfs >/dev/null 2>&1; then
		return
	fi

	export DEBIAN_FRONTEND=noninteractive
	apt-get update
	apt-get install -y nbd-client nfs-kernel-server
}

mount_options() {
	if [ "$WRITABLE" = "true" ]; then
		echo "rw"
		return
	fi

	if [ "$FS_TYPE" = "ext4" ]; then
		echo "ro,noload"
		return
	fi

	echo "ro"
}

export_options() {
	if [ "$WRITABLE" = "true" ]; then
		echo "rw"
		return
	fi

	echo "ro"
}

install_packages
modprobe nbd max_part=16 || true

if nbd-client -c "$NBD_DEVICE" >/dev/null 2>&1; then
	nbd-client -d "$NBD_DEVICE" >/dev/null 2>&1 || true
fi

nbd-client 127.0.0.1 "$NBD_PORT" "$NBD_DEVICE"

DETECTED_TYPE="$(blkid -o value -s TYPE "$NBD_DEVICE" 2>/dev/null || true)"
if [ -z "$DETECTED_TYPE" ]; then
	echo "Unable to detect filesystem type for $NBD_DEVICE" >&2
	exit 1
fi

if [ "$FS_TYPE" = "" ] || [ "$FS_TYPE" = "auto" ]; then
	FS_TYPE="$DETECTED_TYPE"
elif [ "$DETECTED_TYPE" != "$FS_TYPE" ]; then
	echo "Unexpected filesystem type for $NBD_DEVICE: got $DETECTED_TYPE, expected $FS_TYPE" >&2
	exit 1
fi

mkdir -p "$MOUNT_POINT"
if findmnt --noheadings --mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
	umount "$MOUNT_POINT"
fi

mount -t "$FS_TYPE" -o "$(mount_options)" "$NBD_DEVICE" "$MOUNT_POINT"

mkdir -p "$(dirname "$EXPORT_FILE")"
mkdir -p "$EXPORT_ROOT"
cat > "$EXPORT_ROOT_FILE" <<EOF
$EXPORT_ROOT 127.0.0.1(ro,fsid=0,crossmnt,sync,no_subtree_check,no_root_squash,insecure)
EOF
cat > "$EXPORT_FILE" <<EOF
$MOUNT_POINT 127.0.0.1($(export_options),sync,no_subtree_check,no_root_squash,insecure)
EOF

systemctl enable --now nfs-server >/dev/null 2>&1 || systemctl enable --now nfs-kernel-server >/dev/null 2>&1
exportfs -ra
if [ -w /proc/fs/nfsd/v4_end_grace ]; then
	echo Y > /proc/fs/nfsd/v4_end_grace 2>/dev/null || true
fi

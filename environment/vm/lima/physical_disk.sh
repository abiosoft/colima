#!/usr/bin/env sh
set -eu

NAME="$1"
BACKEND="$2"
SOURCE_DEVICE="$3"
NBD_PORT="$4"
MOUNT_POINT="$5"
FS_TYPE="$6"
WRITABLE="$7"
HOST_ACCESS="$8"

EXPORT_FILE="/etc/exports.d/colima-physical-${NAME}.exports"
EXPORT_ROOT="/mnt/colima/physical"
EXPORT_ROOT_FILE="/etc/exports.d/colima-physical-root.exports"

install_packages() {
	if { [ "$HOST_ACCESS" != "true" ] || command -v exportfs >/dev/null 2>&1; } && { [ "$BACKEND" != "nbd" ] || command -v nbd-client >/dev/null 2>&1; }; then
		return
	fi

	export DEBIAN_FRONTEND=noninteractive
	apt-get update
	packages=
	if [ "$BACKEND" = "nbd" ]; then
		packages="$packages nbd-client"
	fi
	if [ "$HOST_ACCESS" = "true" ]; then
		packages="$packages nfs-kernel-server"
	fi
	apt-get install -y $packages
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

case "$BACKEND" in
nbd)
	modprobe nbd max_part=16 || true

	if nbd-client -c "$SOURCE_DEVICE" >/dev/null 2>&1; then
		nbd-client -d "$SOURCE_DEVICE" >/dev/null 2>&1 || true
	fi

	nbd-client 127.0.0.1 "$NBD_PORT" "$SOURCE_DEVICE"
	;;
vz)
	i=0
	while [ "$i" -lt 40 ]; do
		if [ -e "$SOURCE_DEVICE" ]; then
			break
		fi
		udevadm settle >/dev/null 2>&1 || true
		i=$((i + 1))
		sleep 0.25
	done
	if [ ! -e "$SOURCE_DEVICE" ]; then
		echo "Unable to find VZ block device $SOURCE_DEVICE" >&2
		exit 1
	fi
	;;
*)
	echo "Unsupported physical disk backend $BACKEND" >&2
	exit 1
	;;
esac

DETECTED_TYPE="$(blkid -o value -s TYPE "$SOURCE_DEVICE" 2>/dev/null || true)"
if [ -z "$DETECTED_TYPE" ]; then
	echo "Unable to detect filesystem type for $SOURCE_DEVICE" >&2
	exit 1
fi

if [ "$FS_TYPE" = "" ] || [ "$FS_TYPE" = "auto" ]; then
	FS_TYPE="$DETECTED_TYPE"
elif [ "$DETECTED_TYPE" != "$FS_TYPE" ]; then
	echo "Unexpected filesystem type for $SOURCE_DEVICE: got $DETECTED_TYPE, expected $FS_TYPE" >&2
	exit 1
fi

mkdir -p "$MOUNT_POINT"
if findmnt --noheadings --mountpoint "$MOUNT_POINT" >/dev/null 2>&1; then
	umount "$MOUNT_POINT"
fi

mount -t "$FS_TYPE" -o "$(mount_options)" "$SOURCE_DEVICE" "$MOUNT_POINT"

if [ "$HOST_ACCESS" = "true" ]; then
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
fi

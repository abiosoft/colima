#!/usr/bin/env sh

# Steps:
# 1. Check if directory is already mounted, if yes, skip setup
# 2. Idenify disk e.g. /dev/vdb or /dev/vdc
# 3. Format disk with ext4 if not already formatted
# 4. Label disk with instance id
# 5. Mount disk

DISK_LABEL="lima-{{ .InstanceId }}"
MOUNT_POINT="/mnt/${DISK_LABEL}"

# Detect the disk to use e.g. /dev/vdb or /dev/vdc
DISK="/dev/vdb"
if df -h /mnt/lima-cidata/ | tail -n +2 | grep '^/dev/vdb'; then
	DISK="/dev/vdc"
fi
DISK_PART="${DISK}1"

# Check current mount state before touching the disk.
if findmnt --noheadings --source "$DISK_PART" --target "$MOUNT_POINT" >/dev/null 2>&1; then
	echo "Disk already mounted, skipping setup."
	exit 0
fi

if findmnt --noheadings --target "$MOUNT_POINT" >/dev/null 2>&1; then
	echo "Unexpected disk at mount point."
	exit 1
fi

{{ if .Format }}
echo 'type=83' | sudo sfdisk "$DISK"
mkfs.ext4 "$DISK_PART"
e2label "$DISK_PART" "$DISK_LABEL"
{{ end }}

# mount disk
mkdir -p "$MOUNT_POINT"
mount "$DISK_PART" "$MOUNT_POINT"

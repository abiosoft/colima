#!/usr/bin/env sh

# Steps:
# 1. Check if directory is already mounted, if yes, skip setup
# 2. Idenify disk e.g. /dev/vdb or /dev/vdc
# 3. Format disk with ext4 if not already formatted
# 4. Label disk with instance id
# 5. Mount disk

DISK_LABEL="lima-{{ .InstanceId }}"
MOUNT_POINT="/mnt/${DISK_LABEL}"

# Directory already mounted, skip setup
if [ -d "$MOUNT_POINT" ]; then
    if [ -n "$(find "$DIR" -mindepth 1 -print -quit 2>/dev/null)" ]; then
        echo "Disk already mounted, skipping setup."
        exit 0
    fi
fi

# Detect the disk to use e.g. /dev/vdb or /dev/vdc
DISK="/dev/vdb"
if df -h /mnt/lima-cidata/ | tail -n +2 | grep '^/dev/vdb'; then
    DISK="/dev/vdc"
fi
DISK_PART="${DISK}1"

{{ if .Format }}
echo 'type=83' | sudo sfdisk "$DISK"
mkfs.ext4 "$DISK_PART"
e2label "$DISK_PART" "$DISK_LABEL"
{{ end }}

# mount disk
mkdir -p "$MOUNT_POINT"
mount "$DISK_PART" "$MOUNT_POINT"


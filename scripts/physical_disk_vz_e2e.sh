#!/usr/bin/env bash

set -euo pipefail

PROFILE="${PROFILE:-physical-vz-e2e-$RANDOM}"
COLIMA_BINARY="${COLIMA_BINARY:-./_output/binaries/colima-$(uname)-$(uname -m)}"
LIMA_BIN="${LIMA_BIN:-/tmp/lima-5117/_output/bin/limactl}"
SECURE_PREFIX="${SECURE_PREFIX:-/opt/colima-vz-e2e}"
LIMA_SHARE="${LIMA_SHARE:-}"
LIMA_LIBEXEC="${LIMA_LIBEXEC:-}"
HOST_MOUNT="${HOST_MOUNT:-/Volumes/Colima/$PROFILE}"
TMPDIR_ROOT="${TMPDIR_ROOT:-${TMPDIR:-/tmp}}"
IMAGE_SIZE_MIB="${IMAGE_SIZE_MIB:-256}"
VISUDO="${VISUDO:-/usr/sbin/visudo}"

tmpdir=""
image=""
device=""
whole_device=""
profile_dir=""
sudo_keepalive_pid=""

log() {
	printf '\n## %s\n' "$*"
}

die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

have_mount() {
	mount | grep -F " on $1 " >/dev/null 2>&1
}

cleanup() {
	set +e

	if [ -n "$sudo_keepalive_pid" ]; then
		kill "$sudo_keepalive_pid" >/dev/null 2>&1 || true
	fi

	if [ -x "$COLIMA_BINARY" ]; then
		PATH="$SECURE_PREFIX/bin:$PATH" "$COLIMA_BINARY" -p "$PROFILE" delete -f >/dev/null 2>&1 || true
	fi

	if [ -n "$HOST_MOUNT" ] && have_mount "$HOST_MOUNT"; then
		sudo -n umount "$HOST_MOUNT" >/dev/null 2>&1 || sudo umount "$HOST_MOUNT" >/dev/null 2>&1 || true
	fi
	if [ -n "$HOST_MOUNT" ]; then
		rmdir "$HOST_MOUNT" >/dev/null 2>&1 || sudo -n rmdir "$HOST_MOUNT" >/dev/null 2>&1 || true
	fi

	if [ -n "$whole_device" ]; then
		hdiutil detach "$whole_device" >/dev/null 2>&1 || true
	fi

	if [ -n "$profile_dir" ]; then
		rm -rf "$profile_dir"
	fi
	if [ -n "$tmpdir" ]; then
		rm -rf "$tmpdir"
	fi

	sudo -n rm -f /private/etc/sudoers.d/colima-vz-e2e >/dev/null 2>&1 || true
	sudo -n rm -rf "$SECURE_PREFIX" >/dev/null 2>&1 || true
}

trap cleanup EXIT

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "$1 not found"
}

find_lima_assets() {
	if [ -n "$LIMA_SHARE" ] && [ -n "$LIMA_LIBEXEC" ]; then
		return
	fi

	local limactl_path
	limactl_path="$(command -v limactl || true)"
	if [ -z "$limactl_path" ]; then
		die "limactl not found in PATH and LIMA_SHARE/LIMA_LIBEXEC were not set"
	fi

	local prefix
	prefix="$(cd "$(dirname "$limactl_path")/.." && pwd)"
	LIMA_SHARE="${LIMA_SHARE:-$prefix/share/lima}"
	LIMA_LIBEXEC="${LIMA_LIBEXEC:-$prefix/libexec/lima}"

	[ -d "$LIMA_SHARE" ] || die "Lima share directory not found: $LIMA_SHARE"
	[ -d "$LIMA_LIBEXEC" ] || die "Lima libexec directory not found: $LIMA_LIBEXEC"
}

install_secure_lima() {
	log "installing secure limactl helper"

	[ -x "$LIMA_BIN" ] || die "LIMA_BIN is not executable: $LIMA_BIN"
	local create_help sudoers_help
	create_help="$("$LIMA_BIN" create --help 2>&1)"
	sudoers_help="$("$LIMA_BIN" sudoers --help 2>&1)"
	[[ "$create_help" == *"--block-device"* ]] || die "$LIMA_BIN lacks create --block-device"
	[[ "$sudoers_help" == *"--block-device"* ]] || die "$LIMA_BIN lacks sudoers --block-device"

	find_lima_assets

	sudo -v
	(
		while true; do
			sudo -n true >/dev/null 2>&1 || exit 0
			sleep 60
		done
	) &
	sudo_keepalive_pid="$!"

	sudo install -d -m 0755 "$SECURE_PREFIX/bin" "$SECURE_PREFIX/share" "$SECURE_PREFIX/libexec"
	sudo install -m 0555 "$LIMA_BIN" "$SECURE_PREFIX/bin/limactl"
	if [ -x "$(dirname "$LIMA_BIN")/lima" ]; then
		sudo install -m 0555 "$(dirname "$LIMA_BIN")/lima" "$SECURE_PREFIX/bin/lima"
	fi
	sudo rm -f "$SECURE_PREFIX/share/lima" "$SECURE_PREFIX/libexec/lima"
	sudo ln -s "$LIMA_SHARE" "$SECURE_PREFIX/share/lima"
	sudo ln -s "$LIMA_LIBEXEC" "$SECURE_PREFIX/libexec/lima"

	local sudoers_tmp sudoers_line sudoers_file
	sudoers_tmp="$(mktemp "$tmpdir/sudoers.XXXXXX")"
	sudoers_file="/private/etc/sudoers.d/colima-vz-e2e"
	LIMA_HOME="$tmpdir/lima-home" "$SECURE_PREFIX/bin/limactl" sudoers --block-device >"$sudoers_tmp.full"
	sudoers_line="$(grep 'sudo-open-block-device' "$sudoers_tmp.full" || true)"
	[ -n "$sudoers_line" ] || die "failed to generate sudo-open-block-device sudoers entry"
	{
		printf '# Colima physical disk VZ E2E temporary helper\n'
		printf '%s\n' "$sudoers_line"
	} >"$sudoers_tmp"
	sudo "$VISUDO" -cf "$sudoers_tmp" >/dev/null
	sudo install -o root -g wheel -m 0440 "$sudoers_tmp" "$sudoers_file"

	"$SECURE_PREFIX/bin/limactl" --version
}

create_ext4_image() {
	log "creating ext4 disk image"

	tmpdir="$(mktemp -d "$TMPDIR_ROOT/colima-vz-e2e.XXXXXX")"
	mkdir -p "$tmpdir/lima-home/_config"
	cat >"$tmpdir/lima-home/_config/networks.yaml" <<EOF
paths:
  socketVMNet: ""
  varRun: /private/var/run/lima
  sudoers: /private/etc/sudoers.d/colima-vz-e2e

group: everyone

networks:
  user-v2:
    mode: user-v2
    gateway: 192.168.105.1
    netmask: 255.255.255.0
EOF

	image="$tmpdir/vz-physical-smoke.img"
	truncate -s "${IMAGE_SIZE_MIB}m" "$image"
	perl -e 'use strict; use warnings; my $img=shift; my $size=-s $img; my $sectors=int($size/512); my $start=2048; my $count=$sectors-$start; open my $fh, "+<:raw", $img or die $!; seek $fh, 446, 0; print $fh pack("C C3 C C3 V V", 0, 0,2,0, 0x83, 0xff,0xff,0xff, $start, $count); seek $fh, 510, 0; print $fh "\x55\xaa"; close $fh;' "$image"

	local blocks
	blocks=$((IMAGE_SIZE_MIB * 1024 - 1024))
	mke2fs -q -t ext4 -F -L COLIMA_VZ_E2E -E offset=1048576 "$image" "$blocks"

	local attach_out
	attach_out="$(hdiutil attach -nomount -imagekey diskimage-class=CRawDiskImage "$image")"
	printf '%s\n' "$attach_out"
	device="$(printf '%s\n' "$attach_out" | awk '/Linux/ {print $1; exit}')"
	[ -n "$device" ] || die "failed to find Linux partition in hdiutil output"
	whole_device="/dev/$(basename "$device" | sed -E 's/s[0-9]+$//')"
	diskutil info "$device" | grep -q 'Mounted:.*Not applicable\|Mounted:.*No' || die "$device appears mounted on macOS"
}

write_profile_config() {
	log "writing Colima profile $PROFILE"

	profile_dir="$HOME/.config/colima/$PROFILE"
	mkdir -p "$profile_dir"
	cat >"$profile_dir/colima.yaml" <<EOF
vmType: vz
runtime: none
cpu: 2
memory: 2
portForwarder: ssh
network:
  address: false
  mode: shared
physicalDisks:
  - name: smoke
    device: $device
    fsType: ext4
    writable: true
    mountPoint: /mnt/colima/physical/smoke
    backend: vz
    hostAccess:
      enabled: true
      driver: nfs
      mountPoint: $HOST_MOUNT
EOF
}

run_smoke() {
	log "starting Colima profile $PROFILE"

	[ -x "$COLIMA_BINARY" ] || die "COLIMA_BINARY is not executable: $COLIMA_BINARY"
	local colima
	colima=("$COLIMA_BINARY" -p "$PROFILE")

	PATH="$SECURE_PREFIX/bin:$PATH" "${colima[@]}" start --save-config=false

	log "checking guest mount and read/write"
	PATH="$SECURE_PREFIX/bin:$PATH" "${colima[@]}" ssh -- sh -lc '
set -eu
findmnt --noheadings --mountpoint /mnt/colima/physical/smoke
blkid /dev/disk/by-id/virtio-'"$(basename "$device")"'
printf "guest-to-host\n" | sudo tee /mnt/colima/physical/smoke/guest.txt >/dev/null
test "$(cat /mnt/colima/physical/smoke/guest.txt)" = "guest-to-host"
'

	log "checking host NFS read/write"
	test -f "$HOST_MOUNT/guest.txt"
	test "$(cat "$HOST_MOUNT/guest.txt")" = "guest-to-host"
	printf 'host-to-guest\n' >"$HOST_MOUNT/host.txt"
	PATH="$SECURE_PREFIX/bin:$PATH" "${colima[@]}" ssh -- sh -lc 'test "$(cat /mnt/colima/physical/smoke/host.txt)" = "host-to-guest"'
}

main() {
	require_cmd hdiutil
	require_cmd diskutil
	require_cmd mke2fs
	require_cmd perl
	require_cmd truncate
	require_cmd sudo
	[ -x "$VISUDO" ] || die "visudo not found: $VISUDO"

	create_ext4_image
	install_secure_lima
	write_profile_config
	run_smoke

	log "Native VZ physical disk E2E passed for profile $PROFILE"
}

main "$@"

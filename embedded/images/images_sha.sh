#!/usr/bin/env bash

set -eux

BASE_URL=https://github.com/abiosoft/colima-core/releases/download
BASE_FILENAME=ubuntu-24.04-minimal-cloudimg
VERSION=v0.6.10
RUNTIMES="none docker containerd incus"
ARCHS="arm64 amd64"

DIR="$(dirname $0)"
FILE="${DIR}/images.txt"

# reset output files
echo >$FILE

for arch in ${ARCHS}; do
    for runtime in ${RUNTIMES}; do
        URL="${BASE_URL}/${VERSION}/${BASE_FILENAME}-${arch}-${runtime}.qcow2"
        SHA="$(curl -sL ${URL}.sha256sum)"
        echo "$arch $runtime ${URL} ${SHA}" >>$FILE
    done
done

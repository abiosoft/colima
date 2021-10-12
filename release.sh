#!/usr/bin/env sh

set -ex

VERSION="$(git describe --tags)"
if [ -n "$1" ]; then
    VERSION="$1"
    shift
fi

REVISION="$(git rev-parse HEAD)"
PACKAGE="github.com/abiosoft/colima/config"

mkdir -p _output

go build \
    -ldflags "-X ${PACKAGE}.appVersion=${VERSION} -X ${PACKAGE}.revision=${REVISION}" \
    -o _output/colima-amd64 \
    ./cmd/colima

if [ -n "$GITHUB" ]; then
    gh release create "$VERSION" --title "$VERSION" "$@" _output/colima*
fi

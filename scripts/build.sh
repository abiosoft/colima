#!/usr/bin/env sh

set -ex

VERSION="$(git describe --tags --always)"
REVISION="$(git rev-parse HEAD)"
PACKAGE="github.com/abiosoft/colima/config"

OUTPUT_DIR=_output/binaries
mkdir -p "$OUTPUT_DIR"

OUTPUT_BIN="colima-${OS}-${ARCH}"

go build \
    -ldflags "-X ${PACKAGE}.appVersion=${VERSION} -X ${PACKAGE}.revision=${REVISION}" \
    -o "$OUTPUT_DIR/$OUTPUT_BIN" \
    ./cmd/colima

# sha256sum is not on macOS by default, fixable with `brew install coreutils`
cd "${OUTPUT_DIR}" && sha256sum "${OUTPUT_BIN}" >"${OUTPUT_BIN}.sha256sum"

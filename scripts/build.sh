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

# sha256sum is not on macOS by default, use shasum if missing
SHA256SUM=sha256sum
if [[ "$OSTYPE" == "darwin"* ]]; then
    if ! command -v "${SHA256SUM}" &>/dev/null; then
        SHA256SUM="shasum -a 256"
    fi
fi

cd "${OUTPUT_DIR}" && ${SHA256SUM} "${OUTPUT_BIN}" >"${OUTPUT_BIN}.sha256sum"

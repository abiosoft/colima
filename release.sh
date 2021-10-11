#!/usr/bin/env sh

VERSION="$(git describe --tags)"
REVISION="$(git rev-parse HEAD)"
PACKAGE="github.com/abiosoft/colima/config"

mkdir -p _output/amd64

go build \
    -ldflags "-X ${PACKAGE}.appVersion=${VERSION} -X ${PACKAGE}.revision=${REVISION}" \
    -o _output/amd64/colima \
    ./cmd/colima

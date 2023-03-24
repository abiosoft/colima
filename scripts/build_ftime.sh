#!/usr/bin/env sh

set -ex

export EMBED_DIR=$PWD/embedded/ftime
export SRC_DIR=$PWD/share/ftime


move_to_embed_dir() (
    mkdir -p "$EMBED_DIR"
    cp "$SRC_DIR"/zig-out/bin/ftime "$EMBED_DIR"/ftime_"$1"
)

build() (
    cd "$SRC_DIR"

    zig build -Drelease-small -Dtarget=x86_64-linux
    move_to_embed_dir x86_64
    rm -rf zig-out

    zig build -Drelease-small -Dtarget=aarch64-linux
    move_to_embed_dir aarch64
    rm -rf zig-out
)

test_archives() (
    if diff $EMBED_DIR/ftime_x86_64 $EMBED_DIR/ftime_aarch64; then
        echo binary is same for both arch
        exit 1
    fi
)

build
test_archives

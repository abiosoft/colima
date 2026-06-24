#!/usr/bin/env sh

set -ex

export DIR_BUILD=$PWD/_build/network
export DIR_VMNET=$DIR_BUILD/socket_vmnet
export EMBED_DIR=$PWD/embedded/network

clone() (
    if [ ! -d "$2" ]; then
        git clone "$1" "$2"
    fi
)

mkdir -p "$DIR_BUILD"
clone https://github.com/lima-vm/socket_vmnet.git "$DIR_VMNET"

move_to_embed_dir() (
    mkdir -p "$EMBED_DIR"/vmnet/bin
    cp "$DIR_VMNET"/socket_vmnet "$DIR_VMNET"/socket_vmnet_client "$EMBED_DIR"/vmnet/bin
    cd "$EMBED_DIR"/vmnet && tar cvfz "$EMBED_DIR"/vmnet_"${1}".tar.gz bin/socket_vmnet bin/socket_vmnet_client
    rm -rf "$EMBED_DIR"/vmnet
)

build_x86_64() (
    cd "$DIR_VMNET"

    # pinning to a commit for consistency
    git checkout v1.1.5
    make ARCH=x86_64

    move_to_embed_dir x86_64

    # cleanup
    make clean
)

build_arm64() (
    cd "$DIR_VMNET"

    # pinning to a commit for consistency
    git checkout v1.1.5
    make ARCH=arm64
    move_to_embed_dir arm64

    # cleanup
    make clean
)

test_archives() (
    TEMP_DIR=/tmp/colima-test-archives
    rm -rf $TEMP_DIR
    mkdir -p $TEMP_DIR/x86 $TEMP_DIR/arm
    (
        cp "$EMBED_DIR"/vmnet_x86_64.tar.gz $TEMP_DIR/x86
        cd $TEMP_DIR/x86 && tar xvfz vmnet_x86_64.tar.gz
    )
    (
        cp "$EMBED_DIR"/vmnet_arm64.tar.gz $TEMP_DIR/arm
        cd $TEMP_DIR/arm && tar xvfz vmnet_arm64.tar.gz
    )

    assert_not_equal() (
        if diff $TEMP_DIR/x86/"$1" $TEMP_DIR/arm/"$1"; then
            echo "$1" is same for both arch
            exit 1
        fi
    )

    assert_not_equal bin/socket_vmnet
    assert_not_equal bin/socket_vmnet_client
)

build_x86_64
build_arm64
test_archives

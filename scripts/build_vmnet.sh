#!/usr/bin/env sh

set -ex

export DIR_BUILD=$PWD/_build/network
export DIR_VMNET=$DIR_BUILD/vde_vmnet
export DIR_VDE=$DIR_BUILD/vde-2
export EMBED_DIR=$PWD/embedded/network

clone() (
    if [ ! -d "$2" ]; then
        git clone "$1" "$2"
    fi
)

mkdir -p "$DIR_BUILD"
clone https://github.com/lima-vm/vde_vmnet.git "$DIR_VMNET"
clone https://github.com/virtualsquare/vde-2.git "$DIR_VDE"

move_to_embed_dir() (
    mkdir -p "$EMBED_DIR"/vmnet/bin "$EMBED_DIR"/vmnet/lib
    cp "$PREFIX"/bin/vde_vmnet "$EMBED_DIR"/vmnet/bin
    cp "$PREFIX"/lib/libvdeplug.3.dylib "$EMBED_DIR"/vmnet/lib
    cd "$EMBED_DIR"/vmnet && tar cvfz "$EMBED_DIR"/vmnet_"${1}".tar.gz bin/vde_vmnet lib/libvdeplug.3.dylib
    rm -rf "$EMBED_DIR"/vmnet
    sudo rm -rf /opt/colima
)

build_x86_64() (
    # shellcheck disable=SC2030
    export PREFIX=/opt/colima
    sudo rm -rf $PREFIX
    sudo mkdir -p $PREFIX

    # shellcheck disable=SC2155
    export SDKROOT=$(xcrun --sdk macosx --show-sdk-path)
    export CC=$(xcrun --sdk macosx --find clang)
    export CXX=$(xcrun --sdk macosx --find clang++)
    export CFLAGS="-arch x86_64 -isysroot $SDKROOT -Wno-error=implicit-function-declaration"
    export CXXFLAGS=$CFLAGS

    # vde-2
    (
        cd "$DIR_VDE"
        # Dec 12, 2021
        git checkout 74278b9b7cf816f0356181f387012fdeb6d65b52
        autoreconf -fis
        # compile for x86_64
        ./configure --prefix=$PREFIX --host=x86_64-apple-darwin

        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install
        # cleanup
        make distclean
    )

    # vde_vmnet
    (
        cd "$DIR_VMNET"
        make PREFIX=$PREFIX ARCH=x86_64
        sudo make PREFIX=$PREFIX ARCH=x86_64 install.bin
        # cleanup
        rm -f vde_vmnet *.o
    )

    move_to_embed_dir x86_64
)

build_arm64() (
    # shellcheck disable=SC2031
    export PREFIX=/opt/colima
    sudo mkdir -p $PREFIX

    # shellcheck disable=SC2155
    export SDKROOT=$(xcrun --sdk macosx --show-sdk-path)
    export CC=$(xcrun --sdk macosx --find clang)
    export CXX=$(xcrun --sdk macosx --find clang++)
    export CFLAGS="-arch arm64e -isysroot $SDKROOT -Wno-error=implicit-function-declaration"
    export CXXFLAGS=$CFLAGS

    # vde-2
    (
        cd "$DIR_VDE"
        # Dec 12, 2021
        git checkout 74278b9b7cf816f0356181f387012fdeb6d65b52
        autoreconf -fis
        # compile for arm64
        ./configure --prefix=$PREFIX --host=arm-apple-darwin

        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install
        # cleanup
        make distclean
    )

    # vde_vmnet
    (
        cd "$DIR_VMNET"
        make PREFIX=$PREFIX ARCH=arm64
        sudo make PREFIX=$PREFIX ARCH=arm64 install.bin
    )

    move_to_embed_dir arm64
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

    assert_not_equal lib/libvdeplug.3.dylib
    assert_not_equal bin/vde_vmnet
)

build_x86_64
build_arm64
test_archives

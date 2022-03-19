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

mkdir -p $DIR_BUILD
clone https://github.com/lima-vm/vde_vmnet.git $DIR_VMNET
clone https://github.com/virtualsquare/vde-2.git $DIR_VDE

build_x86_64() (
    export PREFIX=/opt/colima
    sudo rm -rf $PREFIX
    sudo mkdir -p $PREFIX

    # vde-2
    (
        cd $DIR_VDE
        # Dec 12, 2021
        git checkout 74278b9b7cf816f0356181f387012fdeb6d65b52
        autoreconf -fis
        # compile for x86_64
        ./configure --prefix=$PREFIX

        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install
        # cleanup
        make distclean
    )

    # vde_vmnet
    (
        cd $DIR_VMNET
        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install.bin
    )

    # copy to embed directory
    (
        mkdir -p $EMBED_DIR/vmnet/bin $EMBED_DIR/vmnet/lib
        cp $PREFIX/bin/vde_vmnet $EMBED_DIR/vmnet/bin
        cp $PREFIX/lib/libvdeplug.3.dylib $EMBED_DIR/vmnet/lib
        cd $EMBED_DIR/vmnet && tar cvfz $EMBED_DIR/vmnet_x86_64.tar.gz bin/vde_vmnet lib/libvdeplug.3.dylib
        rm -rf $EMBED_DIR/vmnet
        sudo rm -rf /opt/colima
    )
)

build_arm64() (
    export PREFIX=/opt/colima
    sudo mkdir -p $PREFIX

    export SDKROOT=$(xcrun --sdk macosx --show-sdk-path)
    export CC=$(xcrun --sdk macosx --find clang)
    export CXX=$(xcrun --sdk macosx --find clang++)
    export CFLAGS="-arch arm64e -isysroot $SDKROOT -Wno-error=implicit-function-declaration"
    export CXXFLAGS=$CFLAGS

    # vde-2
    (
        cd $DIR_VDE
        # Dec 12, 2021
        git checkout 74278b9b7cf816f0356181f387012fdeb6d65b52
        autoreconf -fis
        # compile for arm64
        ./configure --prefix=$PREFIX --host=arm-apple-darwin --target=arm-apple-darwin --build=x86_64-apple-darwin

        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install
        # cleanup
        make distclean
    )

    # vde_vmnet
    (
        cd $DIR_VMNET
        make PREFIX=$PREFIX
        sudo make PREFIX=$PREFIX install.bin
    )

    # copy to embed directory
    (
        mkdir -p $EMBED_DIR/vmnet/bin $EMBED_DIR/vmnet/lib
        cp $PREFIX/bin/vde_vmnet $EMBED_DIR/vmnet/bin
        cp $PREFIX/lib/libvdeplug.3.dylib $EMBED_DIR/vmnet/lib
        cd $EMBED_DIR/vmnet && tar cvfz $EMBED_DIR/vmnet_arm64.tar.gz bin/vde_vmnet lib/libvdeplug.3.dylib
        rm -rf $EMBED_DIR/vmnet
        sudo rm -rf /opt/colima
    )

)

build_x86_64
build_arm64

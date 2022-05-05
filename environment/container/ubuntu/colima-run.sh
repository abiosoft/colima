#!/usr/bin/env bash

CMD=$(basename "$0")

if [ "$CMD" == "colima" ]; then
    if [ -z "$1" ]; then
        sudo chroot /host su - "$USER"
    else
        sudo chroot /host "$@"
    fi
else
    sudo chroot /host "$CMD" "$@"
fi

exit $?
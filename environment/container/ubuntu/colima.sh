#!/usr/bin/env bash

CMD=$(basename "$0")

if [ "$CMD" == "colima" ]; then
    if [ -z "$1" ]; then
        sudo chroot /host su - "$USER"
    else
        sudo chroot /host run-as "$USER" "$PWD" "$@"
    fi
else
    sudo chroot /host run-as "$USER" "$PWD" "$CMD" "$@"
fi

exit $?
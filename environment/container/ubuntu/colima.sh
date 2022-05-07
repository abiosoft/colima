#!/usr/bin/env bash

set -e

if [ ! -f /host/usr/bin/run-as ]; then
    cat <<EOF | sudo tee /host/usr/bin/run-as >/dev/null
#!/usr/bin/env sh
set -u

USER="\$1"
shift

WD="\$1"
shift

cd "\$WD" 2> /dev/null || echo > /dev/null

sudo -u "\$USER" "\$@"
EOF
    sudo chmod 700 /host/usr/bin/run-as
fi

CMD=$(basename "$0")
WD="$PWD"

if [ "$CMD" == "colima" ]; then
    if [ -z "$1" ]; then
        sudo chroot /host su - "$USER"
    else
        sudo chroot /host run-as "$USER" "$WD" "$@"
    fi
else
    sudo chroot /host run-as "$USER" "$WD" "$CMD" "$@"
fi

exit $?

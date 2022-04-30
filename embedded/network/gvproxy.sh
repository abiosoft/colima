#!/usr/bin/env sh

FILE=/etc/network/interfaces

IFACE=$(ifconfig -a | grep 'HWaddr #{.MacAddress}}' | awk -F' ' '{print $1}')

if grep -q "^auto $IFACE" "$FILE"; then
    service networking restart
    exit $?
fi

cat >> $FILE <<EOF
auto $IFACE
iface $IFACE inet dhcp
EOF

service networking restart
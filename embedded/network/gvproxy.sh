#!/usr/bin/env sh

FILE=/etc/network/interfaces

IFACE=$(ifconfig -a | grep 'HWaddr #{.MacAddress}}' | awk -F' ' '{print $1}')

if grep -q "^auto $IFACE" "$FILE"; then
    service networking restart
    exit $?
fi

cat >>$FILE <<EOF
auto $IFACE
iface $IFACE inet static
  address #{.IPAddress}}
  netmask 255.255.255.0
  gateway #{.Gateway}}
  metric 200
EOF

service networking restart

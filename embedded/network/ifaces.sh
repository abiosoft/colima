#!/usr/bin/env sh

FILE=/etc/network/interfaces
BACKUP="$FILE.bak"

# this only happens one in the beginning
if [ ! -f "$BACKUP" ]; then
    cp "$FILE" "$BACKUP"
fi

# reset the network file
cp "$BACKUP" "$FILE"

written() (
    grep -q "^auto ${1}" "${FILE}"
)

vmnet() (
    IFACE="#{.Vmnet.Interface}}"

    if written $IFACE; then exit 0; fi

    cat >>$FILE <<EOF
auto $IFACE
iface $IFACE inet dhcp

EOF
)

gvproxy() (
    IFACE=$(ifconfig -a | grep 'HWaddr #{.GVProxy.MacAddress}}' | awk -F' ' '{print $1}')

    if written $IFACE; then exit; fi

    cat >>$FILE <<EOF
auto $IFACE
iface $IFACE inet static
  address #{.GVProxy.IPAddress}}
  netmask 255.255.255.0
  gateway #{.GVProxy.Gateway}}
  metric 200

EOF
)

#{if .Vmnet.Enabled}}vmnet#{end}}
#{if .GVProxy.Enabled}}gvproxy#{end}}
service networking restart

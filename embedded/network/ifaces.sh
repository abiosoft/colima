#!/usr/bin/env sh
# set -x

FILE=/etc/network/interfaces
BACKUP="$FILE.bak"

# this only happens one in the beginning
if [ ! -f "$BACKUP" ]; then
    cp "$FILE" "$BACKUP"
fi

# reset the network file
cp "$BACKUP" "$FILE"

validate_ip() {
    # Add basic IP address validation, make sure it is in 192.168.106.0/24 subnet
    echo $1 | grep -q -E "^192.168.106.(([01]?[0-9]{1,2})|(2[0-4][0-9])|(25[0-5]))$"
    return $?
}

update_iface_to_static() {
    # update the interface from dhcp to static ip address
    local IFACE=$1
    local IP_ADDRESS=$2
    local FILE=$3

sed -i "/iface $IFACE inet dhcp/d" $FILE
sed -i "/^auto $IFACE/a iface $IFACE inet static\n  address $IP_ADDRESS\n  netmask 255.255.255.0\n  gateway 192.168.106.1\n" $FILE
}

set_iface_to_dhcp() {
    # update interface to using dhcp
    local IFACE=$1

    cat >>$FILE <<EOF
auto $IFACE
iface $IFACE inet dhcp

EOF
}

set_iface_to_static() {
    local IFACE=$1
    local IP_ADDRESS=$2

# default subnet is 192.168.106.0/24
# default gateway is 192.168.106.1

    cat >>$FILE <<EOF
auto $IFACE
iface $IFACE inet static
  address $IP_ADDRESS
  netmask 255.255.255.0
  gateway 192.168.106.1

EOF
}

written() (
    grep -q "^auto ${1}" "${FILE}"
)

vmnet() (
    IFACE="#{.Vmnet.Interface}}"
    if written $IFACE; then
        if [ "$IFACE" == "col0" ]; then
            update_iface_to_static $IFACE $COLIMA_IP $FILE
        fi
        exit 0;
    fi

    if [ "$IFACE" == "col0" ]; then
        if [ -n "$COLIMA_IP" ]; then
            if validate_ip "$COLIMA_IP"; then
                set_iface_to_static $IFACE $COLIMA_IP
            else
                set_iface_to_dhcp
            fi
        else
            set_iface_to_dhcp
        fi
    else
        set_iface_to_dhcp
    fi
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

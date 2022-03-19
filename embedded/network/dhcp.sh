#!/usr/bin/env sh
mkdir -p /etc/udhcpc
touch /etc/udhcpc/udhcpc.conf

if ! grep -q 'NO_GATEWAY' /etc/udhcpc/udhcpc.conf >/dev/null; then
    echo 'NO_GATEWAY="eth0"' >>/etc/udhcpc/udhcpc.conf
fi

kill -s SIGUSR2 $(cat /var/run/udhcpc.eth0.pid) # force DHCP release
kill -s SIGUSR1 $(cat /var/run/udhcpc.eth0.pid) # force DHCP reconfigure

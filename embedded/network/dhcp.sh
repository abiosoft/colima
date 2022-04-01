#!/usr/bin/env sh
mkdir -p /etc/udhcpc
touch /etc/udhcpc/udhcpc.conf

echo 'NO_GATEWAY="{{.Interface}}"' >/etc/udhcpc/udhcpc.conf

kill -s SIGUSR2 $(cat /var/run/udhcpc.{{.Interface}}.pid) # force DHCP release
kill -s SIGUSR1 $(cat /var/run/udhcpc.{{.Interface}}.pid) # force DHCP reconfigure

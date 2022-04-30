#!/usr/bin/env sh
mkdir -p /etc/udhcpc
echo > /etc/udhcpc/udhcpc.conf

#{range .Interfaces}}
echo 'NO_GATEWAY="#{.}}"' >>/etc/udhcpc/udhcpc.conf
kill -s SIGUSR2 $(cat /var/run/udhcpc.#{.}}.pid) # force DHCP release
kill -s SIGUSR1 $(cat /var/run/udhcpc.#{.}}.pid) # force DHCP reconfigure
sleep 2
#{end}}
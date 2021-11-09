#!/usr/bin/env bash

ssh -p "{{.SSHPort}}" \
    -l "{{.VMUser}}" \
    -i ~/.lima/_config/user \
    -o IdentitiesOnly=yes \
    -F /dev/null \
    -o NoHostAuthenticationForLocalhost=yes \
    "127.0.0.1" \
    sudo mkdir -p /run/host-services

ssh -p "{{.SSHPort}}" \
    -l "{{.VMUser}}" \
    -i ~/.lima/_config/user \
    -o IdentitiesOnly=yes \
    -F /dev/null \
    -o NoHostAuthenticationForLocalhost=yes \
    "127.0.0.1" \
    sudo chown {{.VMUser}}:{{.VMUser}} /run/host-services

rm -rf "{{.SocketFile}}"
ssh -p "{{.SSHPort}}" \
    -l "{{.VMUser}}" \
    -i ~/.lima/_config/user \
    -o IdentitiesOnly=yes \
    -F /dev/null \
    -o NoHostAuthenticationForLocalhost=yes \
    -L "{{.SocketFile}}:/var/run/docker.sock" \
    -R "/run/host-services/ssh-auth.sock:$SSH_AUTH_SOCK" \
    -N "127.0.0.1"
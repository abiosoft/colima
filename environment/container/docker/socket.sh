#!/usr/bin/env bash
rm -rf "{{.SocketFile}}"
ssh -p "{{.SSHPort}}" \
    -l "{{.VMUser}}" \
    -i ~/.lima/_config/user \
    -o IdentitiesOnly=yes \
    -F /dev/null \
    -o NoHostAuthenticationForLocalhost=yes \
    -L "{{.SocketFile}}:/var/run/docker.sock" \
    -N "127.0.0.1"
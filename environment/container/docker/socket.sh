#!/usr/bin/env bash
#
# Forwards docker socket from the VM to the host and, when valid, $SSH_AUTH_SOCK to /run/host-services/ssh-auth.sock

#######################################
# Determine whether |SSH_AUTH_SOCK| is set and pointing to a socket.
# Globals:
#   SSH_AUTH_SOCK
# Arguments:
#   None
# Returns: failure when $SSH_AUTH_SOCK is either not set or not a socket.
#######################################
function ssh_auth_socket_is_valid() {
  if [[ -z $SSH_AUTH_SOCK || ! -S $SSH_AUTH_SOCK ]]; then
    return 1
  fi
}

#######################################
# Prepare local sockets
# Globals:
#   SSH_AUTH_SOCK
# Arguments:
#   None
#######################################
function prep_local_sockets() {
  rm -rf "{{.SocketFile}}"
}

#######################################
# Remove remote host_services directory
# Globals:
#   None
# Arguments:
#   None
#######################################
function rm_remote_host_services() {
  if ! ssh_auth_socket_is_valid; then
    return
  fi

  local -a ssh_cmd=(
    ssh -p "{{.SSHPort}}"
    -l "{{.VMUser}}"
    -i ~/.lima/_config/user
    -o IdentitiesOnly=yes
    -F /dev/null
    -o NoHostAuthenticationForLocalhost=yes
    "127.0.0.1"
    sudo rm -rf /var/run/host-services
  )
  "${ssh_cmd[@]}"
}

#######################################
# Install the remote host_services directory
# Globals:
#   None
# Arguments:
#   None
#######################################
function install_remote_host_services() {
  if ! ssh_auth_socket_is_valid; then
    return
  fi

  local -a ssh_cmd=(
    ssh -p "{{.SSHPort}}"
    -l "{{.VMUser}}"
    -i ~/.lima/_config/user
    -o IdentitiesOnly=yes
    -F /dev/null
    -o NoHostAuthenticationForLocalhost=yes
    "127.0.0.1"
    sudo install -d -o "{{.VMUser}}" /var/run/host-services
  )
  "${ssh_cmd[@]}"
}

#######################################
# Ensures that the remote directory for receiving the |SSH_AUTH_SOCK| is prepared (owned by $USER)
# Globals:
#   None
# Arguments:
#   None
#######################################
function prep_remote_sockets() {
  rm_remote_host_services
  install_remote_host_services
}

#######################################
# Forward sockets docker -> local and auth -> remote
# Globals:
#   None
# Arguments:
#   None
#######################################
function forward_sockets() {
  local -a ssh_cmd=(
    ssh -p "{{.SSHPort}}"
    -l "{{.VMUser}}"
    -i ~/.lima/_config/user
    -o IdentitiesOnly=yes
    -F /dev/null
    -o NoHostAuthenticationForLocalhost=yes
    -L "{{.SocketFile}}:/var/run/docker.sock"
  )
  if ssh_auth_socket_is_valid; then
    ssh_cmd+=(
      -R "/run/host-services/ssh-auth.sock:${SSH_AUTH_SOCK}"
    )
  fi

  exec "${ssh_cmd[@]}" -N "127.0.0.1"
}

function main() {
  prep_local_sockets
  prep_remote_sockets
  forward_sockets
}

if ((${#BASH_SOURCE[@]} == 1)); then
  main "$@"
fi

#!/usr/bin/env bash

set -ex

alias colima="$COLIMA_BINARY"
DOCKER_CONTEXT="$(docker info -f '{{json .}}' | jq -r '.ClientInfo.Context')"

OTHER_ARCH="amd64"
if [ "$GOARCH" == "amd64" ]; then
    OTHER_ARCH="arm64"
fi

stage() (
    set +x
    echo
    echo "######################################"
    echo "$@"
    echo "######################################"
    echo
    set -x
)

test_runtime() (
    stage "runtime: $2, arch: $1"

    NAME="itest-$2"
    COLIMA="$COLIMA_BINARY -p $NAME"

    COMMAND="docker"
    if [ "$2" == "containerd" ]; then
       COMMAND="$COLIMA nerdctl --" 
    fi

    # reset
    $COLIMA delete -f

    # start
    $COLIMA start --arch "$1" --runtime "$2"

    # validate
    $COMMAND ps && $COMMAND info

    # validate DNS
    $COLIMA ssh -- nslookup host.docker.internal

    # valid building image
    $COMMAND build integration

    # teardown
    $COLIMA delete -f 
)

test_kubernetes() (
    stage "k8s runtime: $2, arch: $1"

    NAME="itest-$2-k8s"
    COLIMA="$COLIMA_BINARY -p $NAME"

    # reset
    $COLIMA delete -f

    # start
    $COLIMA start --arch "$1" --runtime "$2" --kubernetes

    # short delay
    sleep 5

    # validate
    kubectl cluster-info && kubectl version && kubectl get nodes -o wide

    # teardown
    $COLIMA delete -f
)

test_runtime $GOARCH docker
test_runtime $GOARCH containerd
test_kubernetes $GOARCH docker
test_kubernetes $GOARCH containerd
test_runtime $OTHER_ARCH docker
test_runtime $OTHER_ARCH containerd

if [ -n "$DOCKER_CONTEXT" ]; then
    docker context use "$DOCKER_CONTEXT" || echo # prevent error
fi

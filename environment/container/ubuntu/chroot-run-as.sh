#!/usr/bin/env bash

set -u

USER="$1"
shift

WD="$1"
shift

cd "$WD" 2> /dev/null || echo > /dev/null

sudo -u "$USER" "$@"

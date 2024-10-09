#!/bin/bash
# This script only works for formulas in the homebrew-core.
# It assumes the homebrew-core has been checked out into ./homebrew-core.
# It only needs commit messages, so the checkout can be filtered with tree:0.

set -eu -o pipefail

FORMULA=$1
VERSION=$2

export HOMEBREW_NO_AUTO_UPDATE=1
export HOMEBREW_NO_INSTALL_UPGRADE=1
export HOMEBREW_NO_INSTALL_CLEANUP=1

TAP=lima/tap
if ! brew tap | grep -q "^${TAP}\$"; then
	brew tap-new "$TAP"
fi

# Get the latest commit id for the commit that updated this bottle
SHA=$(git -C homebrew-core log --max-count 1 --grep "^${FORMULA}: update ${VERSION} bottle" --format="%H")
if [[ -z $SHA ]]; then
	echo "${FORMULA} ${VERSION} not found"
	exit 1
fi

OUTPUT="$(brew --repo "$TAP")/Formula/${FORMULA}.rb"
RAW="https://raw.githubusercontent.com/Homebrew/homebrew-core"
curl -s "${RAW}/${SHA}/Formula/${FORMULA::1}/${FORMULA}.rb" -o "$OUTPUT"

if brew ls -1 | grep -q "^${FORMULA}\$"; then
	brew uninstall "$FORMULA" --ignore-dependencies
fi
brew install "${TAP}/${FORMULA}"

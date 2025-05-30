#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Ensure that the GOPATH/{bin,pkg} directory exists. This seems to be not always
# the case in certain environments like Prow. As we will create a symlink against the bin folder we
# need to make sure that the bin directory is present in the GOPATH.
if [ ! -d "$(go env GOPATH)/bin" ]; then mkdir -p "$(go env GOPATH)/bin"; fi
if [ ! -d "$(go env GOPATH)/pkg" ]; then mkdir -p "$(go env GOPATH)/pkg"; fi

VIRTUAL_GOPATH="$(mktemp -d)"
trap 'rm -rf "$VIRTUAL_GOPATH"' EXIT

# Use REPO_ROOT if set, otherwise default to $SCRIPT_DIR/..
TARGET_DIR="${REPO_ROOT:-$SCRIPT_DIR/..}"

# Setup virtual GOPATH
(cd "$TARGET_DIR"; go mod download && "$VGOPATH" -o "$VIRTUAL_GOPATH")

export GOROOT="${GOROOT:-"$(go env GOROOT)"}"
export GOPATH="$VIRTUAL_GOPATH"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"

# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Ensure that if GOPATH is set, the GOPATH/{bin,pkg} directory exists. This seems to be not always
# the case in certain environments like Prow. As we will create a symlink against the bin folder we
# need to make sure that the bin directory is present in the GOPATH.
if [ -n "$GOPATH" ] && [ ! -d "$GOPATH/bin" ]; then mkdir -p "$GOPATH/bin"; fi
if [ -n "$GOPATH" ] && [ ! -d "$GOPATH/pkg" ]; then mkdir -p "$GOPATH/pkg"; fi

VIRTUAL_GOPATH="$(mktemp -d)"
trap 'rm -rf "$VIRTUAL_GOPATH"' EXIT

# Use REPO_ROOT if set, otherwise default to $SCRIPT_DIR/..
TARGET_DIR="${REPO_ROOT:-$SCRIPT_DIR/..}"

# Setup virtual GOPATH
(cd "$TARGET_DIR"; go mod download && "$VGOPATH" -o "$VIRTUAL_GOPATH")

export GOROOT="${GOROOT:-"$(go env GOROOT)"}"
export GOPATH="$VIRTUAL_GOPATH"

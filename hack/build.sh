#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Build"

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
LD_FLAGS="${LD_FLAGS:-$($(dirname $0)/get-build-ld-flags.sh)}"

# This script builds the binaries for the release images and the gardenadm release artifacts.
# -s removes the symbol table and DWARF debug information to reduce the binary size. Stack traces still contain function
# names and line numbers. -trimpath removes local file system paths from the binary to improve reproducibility.
# The local setup (skaffold) does not use this script, so `make debug` keeps the debug information delve needs.
CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GO111MODULE=on \
  go build -trimpath -ldflags "-s $LD_FLAGS" \
  $@

#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Install"

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
LD_FLAGS="${LD_FLAGS:-$($(dirname $0)/get-build-ld-flags.sh)}"

CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GO111MODULE=on \
  go install -ldflags "$LD_FLAGS" \
  $@

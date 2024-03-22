#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Install"

LD_FLAGS="${LD_FLAGS:-$($(dirname $0)/get-build-ld-flags.sh)}"

CGO_ENABLED=0 GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) GO111MODULE=on \
  go install -ldflags "$LD_FLAGS" \
  $@

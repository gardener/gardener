#!/bin/bash
#
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Install"

LD_FLAGS="${LD_FLAGS:-$($(dirname $0)/get-build-ld-flags.sh)}"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on \
  go install -mod=vendor -ldflags "$LD_FLAGS" \
  $@

#!/bin/bash
#
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0
set -e

source "$(dirname $0)/setup-envtest.sh"

echo "> Test"

GO111MODULE=on go test -race -mod=vendor $@ | grep -v 'no test files'

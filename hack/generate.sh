#!/bin/bash
#
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Generate"

GO111MODULE=on go generate -mod=vendor $@

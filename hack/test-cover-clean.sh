#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Test Cover Clean"

find . -name "*.coverprofile" -type f -delete
rm -f test.coverage.html test.coverprofile

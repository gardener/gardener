#!/bin/bash
#
# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0
set -e

echo "> Test Cover Clean"

find . -name "*.coverprofile" -type f -delete
rm -f test.coverage.html test.coverprofile

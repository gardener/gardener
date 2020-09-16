#!/bin/bash
#
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Clean"

for source_tree in $@; do
  find "$(dirname "$source_tree")" -type f -name "zz_*.go" -exec rm '{}' \;
  grep -lr --include="*.go" "//go:generate packr2" . | xargs -I {} packr2 clean "{}/.."
done

#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Check File Names For Invalid Characters"

# Colons are not allowed in file names to avoid "malformed file path" issues with `go get github.com/gardener/gardener@...`

function checkFilenames() {
  (grep -E '[:]' || echo -n "") | while read -r file; do
    echo "File name '$file' contains a colon ':'"
    echo "Please rename the file to remove the colon."
    exit 1
  done
}

# Check for colons in all tracked files
git ls-files | checkFilenames

# Check for colons in all untracked files that are not ignored by .gitignore
git ls-files --others --exclude-standard | checkFilenames

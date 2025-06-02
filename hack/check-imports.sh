#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script uses import-boss to check import restrictions.
# It checks all imports of the given packages (including transitive imports) against rules defined in
# `.import-restrictions` files in each directory.
# An import is allowed if it matches at least one allowed prefix and does not match any forbidden prefixes.
# Note: "" is a prefix of everything
# Also see: https://github.com/kubernetes/code-generator/tree/master/cmd/import-boss

# Usage: `hack/check-imports.sh package [package...]`.

set -o errexit
set -o nounset
set -o pipefail

echo "> Check Imports"

this_module=$(go list -m)

# setup virtual GOPATH
source $(dirname $0)/vgopath-setup.sh

# We need to explicitly pass GO111MODULE=off to import-boss as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
export GO111MODULE=off

packages=()
for p in "$@"; do
  packages+=("$this_module/${p#./}")
done

import-boss -v 1 "$(IFS=, ; echo "${packages[*]}")" 2>&1 | grep -Ev "Ignoring child directory"

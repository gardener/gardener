#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

# We need to explicitly pass GO111MODULE=off to import-boss as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
export GO111MODULE=off

packages=()
for p in "$@" ; do
  packages+=("$this_module/${p#./}")
done

import-boss --include-test-files=true --verify-only --input-dirs "$(IFS=, ; echo "${packages[*]}")"

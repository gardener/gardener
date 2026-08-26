#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "$0")/prepare-envtest.sh"

echo "> Starting envtest"

# change to start-envtest directory to avoid confusing behavior of relative paths
cd "$(dirname "$0")/../test/start-envtest"

go run . "$@"

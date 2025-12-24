#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

CURRENT_DIR="$(dirname $0)"
PROJECT_ROOT="${CURRENT_DIR}"/..
if [ "${PROJECT_ROOT#/}" == "${PROJECT_ROOT}" ]; then
  PROJECT_ROOT="./$PROJECT_ROOT"
fi

pushd "$PROJECT_ROOT" > /dev/null
ROOTS=${ROOTS:-$(
  git grep -l '//go:generate' "$@" | awk '
    {
      if (/\//) { sub(/\/[^\/]+$/, ""); } else { $0 = "."; }
      if (!seen[$0]++) {
        print "github.com/gardener/gardener/" $0
      }
    }
  '
)}
popd > /dev/null

echo "${ROOTS}" | parallel --tag --will-cite 'echo "Generate {}"; go generate {}'

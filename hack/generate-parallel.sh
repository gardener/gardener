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

root_module=$(cd "$PROJECT_ROOT"; go list -m)

pushd "$PROJECT_ROOT" > /dev/null
ROOTS=${ROOTS:-$(git grep --files-with-matches -e '//go:generate' "$@" | \
	xargs -n 1 dirname | \
	sed 's,^,'"$root_module"'/,;' | \
	sort | uniq
)}
popd > /dev/null

read -ra PACKAGES <<< $(echo ${ROOTS})

parallel --will-cite echo Generate {}';' go generate {} ::: ${PACKAGES[@]}

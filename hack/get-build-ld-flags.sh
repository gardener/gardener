#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

PACKAGE_PATH="${1:-k8s.io/component-base}"
VERSION_PATH="${2:-$(dirname $0)/../VERSION}"
PROGRAM_NAME="${3:-Gardener}"
BUILD_DATE="${4:-$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')}"
VERSION_VERSIONFILE="$(cat "$VERSION_PATH")"
VERSION="${EFFECTIVE_VERSION:-$VERSION_VERSIONFILE}"

MAJOR_VERSION=""
MINOR_VERSION=""

if [[ "${VERSION}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)?([-].*)?([+].*)?$ ]]; then
  MAJOR_VERSION=${BASH_REMATCH[1]}
  MINOR_VERSION=${BASH_REMATCH[2]}
  if [[ -n "${BASH_REMATCH[4]}" ]]; then
    MINOR_VERSION+="+"
  fi
fi

# .dockerignore ignores all files unrelevant for build (e.g. docs) to only copy relevant source files to the build
# container. Hence, git will always detect a dirty work tree when building in a container (many deleted files).
# This command filters out all deleted files that are ignored by .dockerignore to only detect changes to relevant files
# as a dirty work tree.
# Additionally, it filters out changes to the `VERSION` file, as this is currently the only way to inject the
# version-to-build in our pipelines (see https://github.com/gardener/cc-utils/issues/431).
TREE_STATE="$([ -z "$(git status --porcelain 2>/dev/null | grep -vf <(git ls-files -o --deleted --ignored --exclude-from=.dockerignore) -e 'VERSION')" ] && echo clean || echo dirty)"

echo "-X $PACKAGE_PATH/version.gitMajor=$MAJOR_VERSION
      -X $PACKAGE_PATH/version.gitMinor=$MINOR_VERSION
      -X $PACKAGE_PATH/version.gitVersion=$VERSION
      -X $PACKAGE_PATH/version.gitTreeState=$TREE_STATE
      -X $PACKAGE_PATH/version.gitCommit=$(git rev-parse --verify HEAD)
      -X $PACKAGE_PATH/version.buildDate=$BUILD_DATE
      -X $PACKAGE_PATH/version/verflag.programName=$PROGRAM_NAME"

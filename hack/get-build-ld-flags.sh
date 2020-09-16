#!/bin/bash -e
#
# SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

SOURCE_REPOSITORY="${1:-github.com/gardener/gardener}"
VERSION_PATH="${2:-$(dirname $0)/../VERSION}"
VERSION_VERSIONFILE="$(cat "$VERSION_PATH")"
VERSION="${EFFECTIVE_VERSION:-$VERSION_VERSIONFILE}"

# .dockerignore ignores all files unrelevant for build (e.g. docs) to only copy relevant source files to the build
# container. Hence, git will always detect a dirty work tree when building in a container (many deleted files).
# This command filters out all deleted files that are ignored by .dockerignore to only detect changes to relevant files
# as a dirty work tree.
# Additionally, it filters out changes to the `VERSION` file, as this is currently the only way to inject the
# version-to-build in our pipelines (see https://github.com/gardener/cc-utils/issues/431).
TREE_STATE="$([ -z "$(git status --porcelain 2>/dev/null | grep -vf <(git ls-files --deleted --ignored --exclude-from=.dockerignore) -e 'VERSION')" ] && echo clean || echo dirty)"

echo "-X $SOURCE_REPOSITORY/pkg/version.gitVersion=$VERSION
      -X $SOURCE_REPOSITORY/pkg/version.gitTreeState=$TREE_STATE
      -X $SOURCE_REPOSITORY/pkg/version.gitCommit=$(git rev-parse --verify HEAD)
      -X $SOURCE_REPOSITORY/pkg/version.buildDate=$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')"

#!/bin/bash -e
#
# Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

SOURCE_REPOSITORY="${1:-github.com/gardener/gardener}"
VERSION_PATH="${2:-$(dirname $0)/../VERSION}"
VERSION_VERSIONFILE="$(cat "$VERSION_PATH")"
VERSION="${EFFECTIVE_VERSION:-$VERSION_VERSIONFILE}"

echo "-X $SOURCE_REPOSITORY/pkg/version.gitVersion=$VERSION
      -X $SOURCE_REPOSITORY/pkg/version.gitTreeState=$([ -z git status --porcelain 2>/dev/null ] && echo clean || echo dirty)
      -X $SOURCE_REPOSITORY/pkg/version.gitCommit=$(git rev-parse --verify HEAD)
      -X $SOURCE_REPOSITORY/pkg/version.buildDate=$(date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$/:\1/g')"

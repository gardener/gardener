#!/usr/bin/env bash
#
# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

set -o errexit
set -o nounset
set -o pipefail

echo "> Test Cover"

REPO_ROOT="$(git rev-parse --show-toplevel)"
COVERPROFILE="$REPO_ROOT/test.coverprofile"
COVERPROFILE_TMP="$REPO_ROOT/test.coverprofile.tmp"
COVERPROFILE_HTML="$REPO_ROOT/test.coverage.html"

trap "rm -rf \"$COVERPROFILE_TMP\"" EXIT ERR INT TERM

GO111MODULE=on go test -cover -coverprofile "$COVERPROFILE_TMP" -race -timeout=2m -mod=vendor $@ | grep -v 'no test files'

cat "$COVERPROFILE_TMP" | grep -vE "\.pb\.go|zz_generated" > "$COVERPROFILE"
go tool cover -html="$COVERPROFILE" -o="$COVERPROFILE_HTML"

go tool cover -func="$COVERPROFILE"

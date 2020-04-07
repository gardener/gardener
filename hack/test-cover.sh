#!/bin/bash
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
set -e

echo "> Test Cover"

"$(dirname $0)"/test.sh -cover $@

COVERPROFILE="$(dirname $0)/../test.coverprofile"
COVERPROFILE_TMP="$(dirname $0)/../test.coverprofile.tmp"
COVERPROFILE_HTML="$(dirname $0)/../test.coverage.html"

echo "mode: set" > "$COVERPROFILE_TMP"
find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> "$COVERPROFILE_TMP"
cat "$COVERPROFILE_TMP" | grep -vE "\.pb\.go|\/test\/|zz_generated" > "$COVERPROFILE"
rm -rf "$COVERPROFILE_TMP"
go tool cover -html="$COVERPROFILE" -o="$COVERPROFILE_HTML"

go tool cover -func="$COVERPROFILE"

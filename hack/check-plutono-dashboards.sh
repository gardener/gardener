#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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


echo "> Checking Plutono dashboards"

function check_dashboards {
  find . -path '*/dashboards/*' -name '*.json' -type f \
  | while IFS= read -r file; do

      jq -c -r '{title: (.title // error("title is not set")),
                 uid:   (.uid   // error("uid is not set"))}
                | if (.uid | length) > 40
                    then error("uid is too long (max length is 40 characters): \(.uid)")
                  elif .title == "" then 
                    error("title is empty")
                  elif .uid == "" then
                    error("UID is empty")
                  else .
                  end
                | "\(input_filename) \(.uid)"' "$file" \
      || { echo "Error: Failure while parsing dashboard: $file" >&2; return 1; }

    done \
  | sort -k 2 | uniq -D -f 1 \
  | { grep . && echo "Error: Dashboards with duplicate UIDs" >&2 && return 1 || return 0; }
}

check_dashboards

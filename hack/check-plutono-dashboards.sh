#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail


echo "> Checking Plutono dashboards"

function check_dashboards {
  find . -path './dev/local-backupbuckets' -prune -o -path '*/dashboards/*' -name '*.json' -type f -print \
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

#!/usr/bin/env bash
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

echo "> Generate monitoring docs"

CURRENT_DIR=$(readlink -f $(dirname $0))
PROJECT_ROOT="$(realpath ${CURRENT_DIR}/..)"

tools="git yaml2json jq"
for t in $tools; do
  if ! which $t &>/dev/null; then
    echo "Tool $t not found in PATH"
    exit 1
  fi
done

cat <<EOF > $PROJECT_ROOT/docs/monitoring/user_alerts.md
# User Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
EOF
cat <<EOF > $PROJECT_ROOT/docs/monitoring/operator_alerts.md
# Operator Alerts
|Alertname|Severity|Type|Description|
|---|---|---|---|
EOF

pushd $PROJECT_ROOT/charts/seed-monitoring/charts/core/charts/prometheus > /dev/null
for file in rules/*.yaml; do
  cat $file | yaml2json | jq -r '
      .groups |
      .[].rules |
      map(select(.labels.visibility == "owner" or .labels.visibility == "all")) |
      map(select(has("alert"))) |
      .[] |
      "|" + .alert + "|" + .labels.severity + "|" + .labels.type + "|" + "`" + .annotations.description + "`" + "|"' >> $PROJECT_ROOT/docs/monitoring/user_alerts.md
  cat $file | yaml2json | jq -r '
      .groups |
      .[].rules |
      map(select(.labels.visibility == "operator" or .labels.visibility == "all")) |
      map(select(has("alert"))) |
      .[] |
      "|" + .alert + "|" + .labels.severity + "|" + .labels.type + "|" + "`" + .annotations.description + "`" + "|"' >> $PROJECT_ROOT/docs/monitoring/operator_alerts.md
done
popd > /dev/null

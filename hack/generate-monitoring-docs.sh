#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

echo "> Generate monitoring docs"

CURRENT_DIR=$(readlink -f $(dirname $0))
PROJECT_ROOT="$(realpath ${CURRENT_DIR}/..)"

tools="git yq"
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

pushd $PROJECT_ROOT/pkg/component/observability/monitoring/charts/seed-monitoring/charts/core/charts/prometheus > /dev/null
for file in rules/worker/*.yaml rules/*.yaml; do
  cat $file | yq -r '
      .groups |
      .[].rules |
      map(select(.labels.visibility == "owner" or .labels.visibility == "all")) |
      map(select(has("alert"))) |
      .[] |
      "|" + .alert + "|" + .labels.severity + "|" + .labels.type + "|" + "`" + .annotations.description + "`" + "|"' >> $PROJECT_ROOT/docs/monitoring/user_alerts.md
  cat $file | yq -r '
      .groups |
      .[].rules |
      map(select(.labels.visibility == "operator" or .labels.visibility == "all")) |
      map(select(has("alert"))) |
      .[] |
      "|" + .alert + "|" + .labels.severity + "|" + .labels.type + "|" + "`" + .annotations.description + "`" + "|"' >> $PROJECT_ROOT/docs/monitoring/operator_alerts.md
done
popd > /dev/null

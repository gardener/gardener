#!/bin/sh
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


# When "renovating" Plutono dashboards, it is useful to export them
# and capture the changes in small git commits, otherwise it is hard
# to review the changes in the dashboard JSON files.
#
# To facilitate this workflow, this script exports a given dashboard
# from Plutono that is port-forwarded to localhost:3000.
#
# This script expects that changes to a dashboard that are made in the Plutono UI
# are saved in the Plutono database and can be exported via the Plutono API.
# To allow this in a _local development environment_, please do the following:
#
# 1. Ignore the Plutono managed resource:
#
# ```bash
# k annotate mr plutono resources.gardener.cloud/ignore=true
# ```
#
# 2. Edit the Plutono deployment and enable the login form:
#
# ```bash
# k edit deployment plutono
# ```
#
# ```yaml
# - name: PL_AUTH_DISABLE_LOGIN_FORM
#   value: "false" # was "true
# ```
#
# 3.The admin password for Plutono can be accessed via:
#
# ```bash
# k get secret -l name=plutono-admin -o json | jq '.items[].data.password | @base64d' -r
# ```
#
# 4. Port-forward Plutono to localhost:3000 and login as admin:
#
# ```bash
# k port-forward deployment/plutono 3000
# ```
#
# 5. Make a copy of the provisioned dashboard you plan to edit, keep the
# suggested title with the " Copy" suffix, get its generated uid (Settings, JSON
# model or copy it from the URL). Make changes in the Plutono UI, save the dashboard,
# export it via this script and create meaningful git commits.
#
# ```bash
# hack/export-dashboard.sh QSl_u1wIz core-dns pkg/component/observability/plutono/dashboards/shoot/owners/worker/coredns-dashboard.json
# ```

cd "$(dirname "$(realpath "$0")")/.." || exit 1

curl -s "http://localhost:3000/api/dashboards/uid/$1" \
| jq --arg original_uid "$2" '
  .dashboard |
  .uid |= $original_uid |
  .title |= sub(" Copy$"; "") |
  del(.id, .iteration, .version)
' > "$3"

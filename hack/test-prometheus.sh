#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Test Prometheus"

echo "Executing shoot Prometheus alert tests"
pushd "$(dirname $0)/../pkg/component/observability/monitoring/charts/seed-monitoring/charts/core/charts/prometheus" > /dev/null
promtool test rules rules-tests/*test.yaml
popd > /dev/null

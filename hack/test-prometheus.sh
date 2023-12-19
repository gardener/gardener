#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Test Prometheus"

echo "Executing Prometheus alert tests"
pushd "$(dirname $0)/../pkg/component/monitoring/charts/seed-monitoring/charts/core/charts/prometheus" > /dev/null
promtool test rules rules-tests/*test.yaml
popd > /dev/null

echo "Executing aggregate Prometheus alert tests"
pushd "$(dirname $0)/../pkg/component/monitoring/charts/bootstrap/aggregate-prometheus-rules-tests" > /dev/null
promtool test rules *test.yaml
popd > /dev/null

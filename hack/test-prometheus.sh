#!/bin/bash
#
# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

echo "> Test Prometheus"

echo "Executing Prometheus alert tests"
pushd "$(dirname $0)/../charts/seed-monitoring/charts/core/charts/prometheus" > /dev/null
promtool test rules rules-tests/*test.yaml
popd > /dev/null

echo "Executing aggregate Prometheus alert tests"
pushd "$(dirname $0)/../charts/seed-bootstrap/aggregate-prometheus-rules-tests" > /dev/null
promtool test rules *test.yaml
popd > /dev/null

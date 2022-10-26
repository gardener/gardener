#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

set -o nounset
set -o pipefail
set -o errexit
set -x

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

# test setup
make kind-ha-up

# export all container logs and events after test execution
trap "
  ( export_logs 'gardener-local-ha';
    export_events_for_kind 'gardener-local-ha'; export_events_for_shoots )
  ( make kind-ha-down )
" EXIT

make gardener-ha-up
make KUBECONFIG=$KUBECONFIG test-e2e-local
make gardener-ha-down

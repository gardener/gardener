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

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu
set_gardener_version_env_variables

# test setup
make kind-ha-multi-zone-up

# export all container logs and events after test execution
trap "
  ( rm -rf $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases; export_logs 'gardener-local-ha-multi-zone';
    export_events_for_kind 'gardener-local-ha-multi-zone'; export_events_for_shoots )
  ( make kind-ha-multi-zone-down;)
" EXIT

# download gardener previous release to perform gardener upgrade tests
$(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_PREVIOUS_RELEASE --download-path $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases

pushd $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_PREVIOUS_RELEASE
cp $KUBECONFIG example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
cp $KUBECONFIG example/gardener-local/kind/ha-multi-zone/kubeconfig
echo "Installing gardener version '$GARDENER_PREVIOUS_RELEASE'"
make gardener-ha-multi-zone-up
popd

echo "Running gardener pre-upgrade tests"
make test-pre-upgrade-ha-multi-zone GARDENER_PREVIOUS_RELEASE=$GARDENER_PREVIOUS_RELEASE GARDENER_NEXT_RELEASE=$GARDENER_NEXT_RELEASE

# downloads and upgrades to GARDENER_NEXT_RELEASE release if GARDENER_NEXT_RELEASE is not same as version mentioned in VERSION file.
# if GARDENER_NEXT_RELEASE is same as version mentioned in VERSION file then it is considered as local release and install gardener from local repo.
echo "Upgrading gardener version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
if [[ -n $GARDENER_NEXT_RELEASE && $GARDENER_NEXT_RELEASE != $VERSION ]]; then
  # download gardener previous release to perform gardener upgrade tests
  $(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_NEXT_RELEASE --download-path $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_NEXT_RELEASE
  pushd $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_NEXT_RELEASE
  cp $KUBECONFIG example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
  cp $KUBECONFIG example/gardener-local/kind/ha-multi-zone/kubeconfig
  make gardener-ha-multi-zone-up
  popd
else
  make gardener-ha-multi-zone-up
fi

wait_until_seed_gets_upgraded "local-ha-multi-zone"

echo "Running gardener post-upgrade tests"
make test-post-upgrade-ha-multi-zone GARDENER_PREVIOUS_RELEASE=$GARDENER_PREVIOUS_RELEASE GARDENER_NEXT_RELEASE=$GARDENER_NEXT_RELEASE

make gardener-ha-multi-zone-down

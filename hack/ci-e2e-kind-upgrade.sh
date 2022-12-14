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

VERSION="$(cat VERSION)"
CLUSTER_NAME=""
SEED_NAME=""

# copy_kubeconfig_from_kubeconfig_env_var copies the kubeconfig to apporiate location based on kind setup
function copy_kubeconfig_from_kubeconfig_env_var() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    cp $KUBECONFIG example/provider-local/seed-kind-ha-single-zone/base/kubeconfig
    cp $KUBECONFIG example/gardener-local/kind/ha-single-zone/kubeconfig
    ;;
  zone)
    cp $KUBECONFIG example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
    cp $KUBECONFIG example/gardener-local/kind/ha-multi-zone/kubeconfig
    ;;
  *)
    cp $KUBECONFIG example/provider-local/seed-kind/base/kubeconfig
    cp $KUBECONFIG example/gardener-local/kind/local/kubeconfig
    ;;
  esac
}

function gardener_up() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make gardener-ha-single-zone-up
    ;;
  zone)
    make gardener-ha-multi-zone-up
    ;;
  *)
    make gardener-up
    ;;
  esac
}

function gardener_down() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make gardener-ha-single-zone-down
    ;;
  zone)
    make gardener-ha-multi-zone-down
    ;;
  *)
    make gardener-down
    ;;
  esac
}

function kind_up() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make kind-ha-single-zone-up
    ;;
  zone)
    make kind-ha-multi-zone-up
    ;;
  *)
    make kind-up
    ;;
  esac
}

function kind_down() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make kind-ha-single-zone-down
    ;;
  zone)
    make kind-ha-multi-zone-down
    ;;
  *)
    make kind-down
    ;;
  esac
}

function install_previous_release() {
  pushd $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_PREVIOUS_RELEASE >/dev/null
  copy_kubeconfig_from_kubeconfig_env_var
  gardener_up
  popd >/dev/null
}

function upgrade_to_next_release() {
  # downloads and upgrades to GARDENER_NEXT_RELEASE release if GARDENER_NEXT_RELEASE is not same as version mentioned in VERSION file.
  # if GARDENER_NEXT_RELEASE is same as version mentioned in VERSION file then it is considered as local release and install gardener from local repo.
  if [[ -n $GARDENER_NEXT_RELEASE && $GARDENER_NEXT_RELEASE != $VERSION ]]; then
    # download gardener previous release to perform gardener upgrade tests
    $(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_NEXT_RELEASE --download-path $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases
    pushd $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_NEXT_RELEASE >/dev/null
    copy_kubeconfig_from_kubeconfig_env_var
    gardener_up
    popd >/dev/null
  else
    gardener_up
  fi

}

function set_gardener_upgrade_version_env_variables() {
  if [[ -z "$GARDENER_PREVIOUS_RELEASE" ]]; then
    export GARDENER_PREVIOUS_RELEASE="$(curl -s https://api.github.com/repos/gardener/gardener/releases/latest | grep tag_name | cut -d '"' -f 4)"
  fi

  if [[ -z "$GARDENER_NEXT_RELEASE" ]]; then
    export GARDENER_NEXT_RELEASE="$VERSION"
  fi
}

function set_cluster_name() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    CLUSTER_NAME="gardener-local-ha-single-zone"
    ;;
  zone)
    CLUSTER_NAME="gardener-local-ha-multi-zone"
    ;;
  *)
    CLUSTER_NAME="gardener-local"
    ;;
  esac
}

function set_seed_name() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    SEED_NAME="local-ha-single-zone"
    ;;
  zone)
    SEED_NAME="local-ha-multi-zone"
    ;;
  *)
    SEED_NAME="local"
    ;;
  esac
}

function wait_until_seed_gets_upgraded() {
  echo "Wait until seed gets upgraded from version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
  kubectl wait seed $1 --timeout=5m \
    --for=jsonpath='{.status.gardener.version}'=$GARDENER_NEXT_RELEASE && condition=gardenletready && condition=extensionsready && condition=bootstrapped
}

clamp_mss_to_pmtu
set_gardener_upgrade_version_env_variables
set_cluster_name
set_seed_name

# download gardener previous release to perform gardener upgrade tests
$(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_PREVIOUS_RELEASE --download-path $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases

# test setup
kind_up

# export all container logs and events after test execution
trap "
( rm -rf $GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases);
( export_logs '$CLUSTER_NAME'; export_events_for_kind '$CLUSTER_NAME'; export_events_for_shoots )
( kind_down;)
" EXIT

echo "Installing gardener version '$GARDENER_PREVIOUS_RELEASE'"
install_previous_release

echo "Running gardener pre-upgrade tests"
make test-pre-upgrade GARDENER_PREVIOUS_RELEASE=$GARDENER_PREVIOUS_RELEASE GARDENER_NEXT_RELEASE=$GARDENER_NEXT_RELEASE

echo "Upgrading gardener version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
upgrade_to_next_release
wait_until_seed_gets_upgraded "$SEED_NAME"

echo "Running gardener post-upgrade tests"
make test-post-upgrade GARDENER_PREVIOUS_RELEASE=$GARDENER_PREVIOUS_RELEASE GARDENER_NEXT_RELEASE=$GARDENER_NEXT_RELEASE

gardener_down

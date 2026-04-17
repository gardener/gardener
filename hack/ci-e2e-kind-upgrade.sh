#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

source $(dirname "${0}")/ci-common.sh

VERSION="$(cat VERSION)"
SEED_NAME="local"

function kind_up() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make kind-multi-node-up
    ;;
  zone)
    make kind-multi-zone-up
    ;;
  *)
    make kind-up
    ;;
  esac
}

function kind_down() {
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
  node)
    make kind-multi-node-down
    ;;
  zone)
    make kind-multi-zone-down
    ;;
  *)
    make kind-down
    ;;
  esac
}

# copy_kubeconfig_files_to_old_gardener_version_folder copies the kubeconfig files from Gardener repository folder
# representing the 'next version' to the Gardener repository folder representing the 'previous version'. This is because
# `make kind*-up` is executed from the folder of the 'next version' before the 'previous version' is downloaded into
# another folder. From this other folder, Gardener gets installed into the existing kind cluster, but this requires the
# kubeconfig files in the worktree.
# This function mirrors the behaviour of ./hack/kind-up.sh (which also copies the kubeconfig of the kind cluster into
# various locations).
function copy_kubeconfig_files_to_old_gardener_version_folder() {
  cp "$KUBECONFIG_RUNTIME_CLUSTER"  "dev-setup/${KUBECONFIG_RUNTIME_CLUSTER#*/dev-setup/}"
  cp "$KUBECONFIG_SEED_CLUSTER"     "dev-setup/${KUBECONFIG_SEED_CLUSTER#*/dev-setup/}"
  cp "$KUBECONFIG_SEED_SECRET_PATH" "dev-setup/${KUBECONFIG_SEED_SECRET_PATH#*/dev-setup/}"
}


function install_previous_release() {
  pushd "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_PREVIOUS_RELEASE" >/dev/null
  copy_kubeconfig_files_to_old_gardener_version_folder
  remove_provider_local_service_controller
  make gardener-up
  popd >/dev/null
}

# TODO(timebertt): remove this after v1.141 has been released.
# In the upgrade tests, `make kind-up` is executed with the new version of Gardener, which includes deploying cloud-controller-manager-local to the kind cluster.
# However, `make gardener-up` is executed with the old version of Gardener, which still includes the service controller in gardener-extension-provider-local.
# This leads to a conflict between cloud-controller-manager-local and service controller, as they both try to reconcile LoadBalancer services.
# This function removes the service controller from the previous version of Gardener to rely on cloud-controller-manager-local throughout the upgrade test.
function remove_provider_local_service_controller() {
  sed -i '/servicecontroller/d' cmd/gardener-extension-provider-local/app/options.go
}

function upgrade_to_next_release() {
  # downloads and upgrades to GARDENER_NEXT_RELEASE release if GARDENER_NEXT_RELEASE is not same as version mentioned in VERSION file.
  # if GARDENER_NEXT_RELEASE is same as version mentioned in VERSION file then it is considered as local release and install gardener from local repo.
  if [[ -n $GARDENER_NEXT_RELEASE && $GARDENER_NEXT_RELEASE != $VERSION ]]; then
    # download gardener next release to perform gardener upgrade tests
    $(dirname "${0}")/download_gardener_source_code.sh --gardener-version "$GARDENER_NEXT_RELEASE" --download-path "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases"
    export GARDENER_NEXT_VERSION="$(cat "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_NEXT_RELEASE/VERSION")"
    pushd "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_NEXT_RELEASE" >/dev/null
    copy_kubeconfig_files_to_old_gardener_version_folder
    make gardener-up
    popd >/dev/null
  else
    export GARDENER_NEXT_VERSION=$VERSION
    make gardener-up
  fi
}

function set_gardener_upgrade_version_env_variables() {
  if [[ -z "$GARDENER_PREVIOUS_RELEASE" ]]; then
    previous_minor_version=$(echo "$VERSION" | awk -F. '{printf("%s.%d", $1, $2-1)}')
    pre_previous_minor_version=$(echo "$previous_minor_version" | awk -F. '{printf("%s.%d", $1, $2-1)}')
    # List all the tags that match the previous minor version pattern
    tag_list=$(git tag -l "${previous_minor_version}.*")
    tag_list_pre=$(git tag -l "${pre_previous_minor_version}.*")

    # Find the most recent tag for the previous minor version
    if [ "$tag_list" ]; then
      export GARDENER_PREVIOUS_RELEASE=$(echo "$tag_list" | tail -n 1)
    # Try to use release branch of previous version as backup
    elif [ "$(git ls-remote https://github.com/gardener/gardener release-"${previous_minor_version}")" ]; then
      export GARDENER_PREVIOUS_RELEASE="release-${previous_minor_version}"
      echo "No tags found for the previous minor version ($VERSION) to upgrade Gardener. Using branch $GARDENER_PREVIOUS_RELEASE instead." >&2
    # If the release branch is found neither, use the tag of the pre previous version as last resort
    elif [ "$tag_list_pre" ]; then
      export GARDENER_PREVIOUS_RELEASE=$(echo "$tag_list_pre" | tail -n 1)
      echo "No tags and branches found for the previous minor version ($VERSION) to upgrade Gardener. Using tag $GARDENER_PREVIOUS_RELEASE instead." >&2
    else
      echo "No tags and release branches found for the previous and pre-previous minor version ($VERSION) to upgrade Gardener." >&2
      exit 1
    fi
  fi

  if [[ -z "$GARDENER_NEXT_RELEASE" ]]; then
    export GARDENER_NEXT_RELEASE="$VERSION"
  fi
}

function run_pre_upgrade_test() {
  local test_command

  if [[ "$SHOOT_FAILURE_TOLERANCE_TYPE" == "node" || "$SHOOT_FAILURE_TOLERANCE_TYPE" == "zone" ]]; then
    test_command="test-e2e-pre-upgrade"
  else
    test_command="test-e2e-non-ha-pre-upgrade"
  fi

  make "$test_command" GARDENER_PREVIOUS_RELEASE="$GARDENER_PREVIOUS_RELEASE" GARDENER_NEXT_RELEASE="$GARDENER_NEXT_RELEASE"
}

function run_post_upgrade_test() {
  local test_command

  if [[ "$SHOOT_FAILURE_TOLERANCE_TYPE" == "node" || "$SHOOT_FAILURE_TOLERANCE_TYPE" == "zone" ]]; then
    test_command="test-e2e-post-upgrade"
  else
    test_command="test-e2e-non-ha-post-upgrade"
  fi

  make "$test_command" GARDENER_PREVIOUS_RELEASE="$GARDENER_PREVIOUS_RELEASE" GARDENER_NEXT_RELEASE="$GARDENER_NEXT_RELEASE"
}

# TODO(rfranzke): Remove this after v1.141 is released and the upgrade tests can be enabled again.
if true; then
  echo "WARNING: The Gardener upgrade tests are not executed in this release because the dev/e2e test setup is currently reworked."
  echo "See https://github.com/gardener/gardener/issues/11958 for more information."
  echo "Skipping the tests."
  echo "After the rework has been finished, this early exit can be removed again (TODO(rfranzke))."
  exit 0
fi

clamp_mss_to_pmtu
set_gardener_upgrade_version_env_variables

# download gardener previous release to perform gardener upgrade tests
$(dirname "${0}")/download_gardener_source_code.sh --gardener-version "$GARDENER_PREVIOUS_RELEASE" --download-path "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases"
export GARDENER_PREVIOUS_VERSION="$(cat "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases/$GARDENER_PREVIOUS_RELEASE/VERSION")"

# test setup
kind_up

# export all container logs and events after test execution
trap "
  ( rm -rf "$GARDENER_RELEASE_DOWNLOAD_PATH/gardener-releases" )
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden extop )
  ( export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for gardenlet seeds shoots; export_events_for_shoots )
  ( make gardener-down )
  ( kind_down )
" EXIT

echo "Installing gardener version '$GARDENER_PREVIOUS_RELEASE'"
install_previous_release

echo "Running gardener pre-upgrade tests"
run_pre_upgrade_test

echo "Upgrading gardener version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
upgrade_to_next_release

echo "Wait until seed '$SEED_NAME' gets upgraded from version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
kubectl wait seed $SEED_NAME --timeout=5m --for=jsonpath="{.status.gardener.version}=$GARDENER_NEXT_RELEASE"
# TIMEOUT has been increased to 1200 (20 minutes) due to the upgrading of Gardener for seed.
# In a single-zone setup, 2 istio-ingressgateway pods will be running, and it will take 9 minutes to complete the rollout.
# In a multi-zone setup, 6 istio-ingressgateway pods will be running, and it will take 18 minutes to complete the rollout.
TIMEOUT=1200 ./hack/usage/wait-for.sh seed "$SEED_NAME" GardenletReady SeedSystemComponentsHealthy ExtensionsReady BackupBucketsReady

# The downtime validator considers downtime after 3 consecutive failures, taking a total of 30 seconds.
# Therefore, we're waiting for double that amount of time (60s) to detect if there is any downtime after the upgrade process.
# By waiting for double the amount of time (60 seconds) post-upgrade, the script accounts for the possibility of missing the last 30-second window,
# thus ensuring that any potential downtime after the post-upgrade is detected.
sleep 60

echo "Running gardener post-upgrade tests"
run_post_upgrade_test

#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

# export all container logs and events after test execution, then tear down both clusters
trap "
  ( git checkout -- dev-setup/gardenadm/resources/base/shoot.yaml )
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export_artifacts_gind )
  ( [[ -s $KUBECONFIG_RUNTIME_CLUSTER ]] && { export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden; } || true )
  ( [[ -s $KUBECONFIG_VIRTUAL_GARDEN_CLUSTER ]] && { export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots shootstates managedseeds controllerinstallations; } || true )
  ( [[ -s $KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER ]] && { export KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER; export_artifacts_for_cluster 'self-hosted-shoot'; } || true )
  ( make gind-down )
  ( make kind-down )
" EXIT

# The restore e2e test requires the 'worker' pool to not run system components until
# https://github.com/gardener/gardener/issues/15951 is fixed. Temporarily disable it here; the EXIT
# trap reverts the file. Mutating this tracked file in place is safe because each Prow job runs in its
# own isolated clonerefs checkout.
#
# TODO(ialidzhikov): Remove once https://github.com/gardener/gardener/issues/15951 is resolved.
yq -i '(.spec.provider.workers[] | select(.name == "worker").systemComponents.allow) = false' \
  dev-setup/gardenadm/resources/base/shoot.yaml

make gind-up GARDENADM_INIT_FLAGS="--log-level=debug" SCENARIO=join

make kind-up
make gardenadm-up SCENARIO=connect-kind

make test-e2e-local-gardenadm-unmanaged-infra-restore

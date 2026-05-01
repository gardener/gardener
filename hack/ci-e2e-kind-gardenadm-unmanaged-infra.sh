#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export_artifacts_gind )
  ( export KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER; export_artifacts_for_cluster 'self-hosted-shoot'; export_resource_yamls_for garden )
  ( export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots managedseeds controllerinstallations )
  ( make seed-down KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER )
  ( make gardenadm-down SCENARIO=connect )
  ( make gind-down )
" EXIT

make gind-up GARDENADM_INIT_FLAGS="--log-level=debug"
make test-e2e-local-gardenadm-unmanaged-infra-initjoin

make gardenadm-up SCENARIO=connect
make test-e2e-local-gardenadm-unmanaged-infra-connect

make seed-up KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
make test-e2e-local-gardenadm-unmanaged-infra-seed

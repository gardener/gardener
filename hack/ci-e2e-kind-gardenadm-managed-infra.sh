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

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden )
  ( [[ -s $KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER ]] && { export KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER; export_artifacts_for_cluster 'self-hosted-shoot'; export_resource_yamls_for garden; } || true )
  ( [[ -s $KUBECONFIG_VIRTUAL_GARDEN_CLUSTER ]] && { export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots managedseeds controllerinstallations; } || true )
  ( [[ -s $KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER ]] && make seed-down KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER || true )
  ( make gardenadm-down SCENARIO=connect )
  ( make kind-down )
" EXIT

make kind-up
make gardenadm-up SCENARIO=managed-infra
make test-e2e-local-gardenadm-managed-infra-bootstrap

make gardenadm-up SCENARIO=connect
make test-e2e-local-gardenadm-managed-infra-connect

make seed-up KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
make test-e2e-local-gardenadm-managed-infra-seed

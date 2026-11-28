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

# The self-hosted shoot doesn't exist anymore after a successful `gardenadm destroy`, hence only export its artifacts if
# it is still reachable.
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden )
  ( ./hack/usage/generate-kubeconfig.sh self-hosted-shoot > $KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER 2>/dev/null &&
    kubectl --kubeconfig $KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER --request-timeout 10s get namespace kube-system >/dev/null 2>&1 &&
    { export KUBECONFIG=$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER; export_artifacts_for_cluster 'self-hosted-shoot'; export_resource_yamls_for garden; } || true )
  ( make gardenadm-down SCENARIO=managed-infra )
  ( make kind-down )
" EXIT

make kind-up
make gardenadm-up SCENARIO=managed-infra
make test-e2e-local-gardenadm-managed-infra-bootstrap
make test-e2e-local-gardenadm-managed-infra-destroy

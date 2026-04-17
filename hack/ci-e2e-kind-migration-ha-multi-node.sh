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

# test setup
make kind-multi-node-up
make kind-multi-node2-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden )
  ( export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for gardenlet seeds shoots; export_events_for_shoots )
  ( export KUBECONFIG=$KUBECONFIG_SEED2_CLUSTER; export_artifacts "gardener-local2" )
  ( make seed-down KUBECONFIG="$KUBECONFIG_SEED2_CLUSTER" )
  ( make operator-seed-down )
  ( make kind-multi-node2-down )
  ( make kind-multi-node-down )
" EXIT

make operator-seed-up
make seed-up KUBECONFIG="$KUBECONFIG_SEED2_CLUSTER"

make test-e2e-local-migration-ha-multi-node

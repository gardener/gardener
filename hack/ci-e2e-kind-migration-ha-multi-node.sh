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

ensure_local_gardener_cloud_hosts

# test setup
make kind-multi-node-up
make kind-multi-node2-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$PWD/example/gardener-local/kind/multi-zone/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden)
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots; export_events_for_shoots)
  ( export KUBECONFIG=$GARDENER_LOCAL2_KUBECONFIG; export_artifacts "gardener-local-multi-node2" )
  ( make kind-multi-node-down )
  ( make kind-multi-node2-down )
" EXIT

make operator-seed-up
make seed-up KUBECONFIG="$GARDENER_LOCAL2_KUBECONFIG"

make test-e2e-local-migration-ha-multi-node

make seed-down KUBECONFIG="$GARDENER_LOCAL2_KUBECONFIG"
make operator-seed-down

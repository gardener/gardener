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
make kind-multi-zone-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$KUBECONFIG_RUNTIME_CLUSTER; export_artifacts 'gardener-local'; export_resource_yamls_for garden )
  ( export KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER; export cluster_name='virtual-garden'; export_resource_yamls_for gardenlet seeds shoots; export_events_for_shoots )
  ( make gardener-down )
  ( make kind-multi-zone-down )
" EXIT

make gardener-up
make test-e2e-local-ha-multi-zone

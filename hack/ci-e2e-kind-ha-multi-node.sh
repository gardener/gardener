#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit
set -x

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

# test setup
make kind-multi-node-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra; export_artifacts_load_balancers )
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/runtime/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden extop )
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for gardenlet seeds shoots; export_events_for_shoots )
  ( make operator-seed-down )
  ( make kind-multi-node-down )
" EXIT

make operator-seed-up
make test-e2e-local-ha-multi-node

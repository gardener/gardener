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
  ( export_artifacts_host_services; export_artifacts_infra )
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/runtime/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden )
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots )
  ( make gardenadm-down SCENARIO=managed-infra )
  ( make kind-single-node-down )
" EXIT

make kind-single-node-up
make gardenadm-up SCENARIO=managed-infra

make test-e2e-local-gardenadm-managed-infra

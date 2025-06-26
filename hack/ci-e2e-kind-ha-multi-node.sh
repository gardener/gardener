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

ensure_glgc_resolves_to_localhost

# test setup
make kind-operator-multi-node-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$PWD/example/gardener-local/kind/operator/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for gardens extop)
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots; export_events_for_shoots)
  ( make kind-operator-multi-node-down )
" EXIT

make operator-seed-up SKAFFOLD_PROFILE=multi-node
make test-e2e-local-ha-multi-node
make operator-seed-down SKAFFOLD_PROFILE=multi-node

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

ensure_glgc_resolves_to_localhost

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$PWD/example/gardener-local/kind/multi-zone/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden)
  ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots; export_events_for_shoots)
  ( make kind-single-node-down )
" EXIT

# managed-infra tests cannot run when there is a gardener-operator deployment (i.e., unmanaged-infra/connect tests must run
# separately). Hence, let's run the managed-infra tests first, then clean them up, and then run the unmanaged-infra/connect
# tests.

# managed infrastructure
make kind-single-node-up
make gardenadm-up SCENARIO=managed-infra

make test-e2e-local-gardenadm-managed-infra

make gardenadm-down SCENARIO=managed-infra
make kind-single-node-down

# unmanaged infrastructure
make kind-single-node-up
make gardenadm-up SCENARIO=unmanaged-infra
make gardenadm-up SCENARIO=connect

make test-e2e-local-gardenadm-unmanaged-infra

make gardenadm-down SCENARIO=connect
make gardenadm-down SCENARIO=unmanaged-infra
make kind-single-node-down

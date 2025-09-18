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

# medium-touch tests cannot run when there is a gardener-operator deployment (i.e., high-touch/connect tests must run
# separately). Hence, let's run the medium-touch tests first, then clean them up, and then run the high-touch/connect
# tests.

# medium-touch
make kind-single-node-up
make gardenadm-up SCENARIO=medium-touch

make test-e2e-local-gardenadm-medium-touch

make gardenadm-down SCENARIO=medium-touch
make kind-single-node-down

# high touch
make kind-single-node-up
make gardenadm-up SCENARIO=high-touch
make gardenadm-up SCENARIO=connect

make test-e2e-local-gardenadm-high-touch

make gardenadm-down SCENARIO=connect
make gardenadm-down SCENARIO=high-touch
make kind-single-node-down

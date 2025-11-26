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
make kind-multi-zone-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$PWD/example/gardener-local/kind/multi-zone/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden)
  ( make kind-multi-zone-down )
" EXIT

make operator-up
make test-e2e-local-operator
make operator-down

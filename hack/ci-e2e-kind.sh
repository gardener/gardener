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

if [[ -n "$IPFAMILY" ]] && [[ "$IPFAMILY" == "ipv6" ]]; then
  make kind-single-node-up

  # export all container logs and events after test execution
  trap "
    ( export_artifacts_host )
    ( export KUBECONFIG=$PWD/example/gardener-local/kind/multi-zone/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden extop)
    ( export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig; export cluster_name='virtual-garden'; export_resource_yamls_for gardenlet seeds shoots; export_events_for_shoots)
    ( make kind-single-node-down )
  " EXIT

  make operator-seed-up
  # TODO(rfranzke): Remove this KUBECONFIG environment variable once the ci-e2e-kind setup is switched to gardener-operator.
  make test-e2e-local KUBECONFIG="$(git rev-parse --show-toplevel)/dev-setup/kubeconfigs/virtual-garden/kubeconfig"
  make operator-seed-down
  exit 0
fi

# test setup
make kind-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host )
  ( export_artifacts "gardener-local" )
  ( make kind-down )
" EXIT

make gardener-up
make test-e2e-local
make gardener-down

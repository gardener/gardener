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
make kind-up
make kind2-up

# export all container logs and events after test execution
trap "
  ( export_artifacts_host_services; export_artifacts_infra )
  ( export KUBECONFIG=$GARDENER_LOCAL_KUBECONFIG; export_artifacts "gardener-local" )
  ( export KUBECONFIG=$GARDENER_LOCAL_KUBECONFIG; export cluster_name='virtual-garden'; export_resource_yamls_for seeds shoots bastions.operations.gardener.cloud etcds leases; export_events_for_shoots )
  ( export KUBECONFIG=$GARDENER_LOCAL2_KUBECONFIG; export_artifacts "gardener-local2" )
  ( make kind-down )
  ( make kind2-down )
" EXIT

make gardener-up
make gardenlet-kind2-up
make test-e2e-local-migration
make gardener-down
make gardenlet-kind2-down

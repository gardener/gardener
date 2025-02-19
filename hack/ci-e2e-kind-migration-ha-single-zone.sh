#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

source $(dirname "${0}")/ci-common.sh

clamp_mss_to_pmtu

ensure_glgc_resolves_to_localhost

# test setup
make kind-ha-single-zone-up
make kind2-ha-single-zone-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$GARDENER_LOCAL_KUBECONFIG; export_artifacts "gardener-local-ha-single-zone" )
  ( export KUBECONFIG=$GARDENER_LOCAL2_KUBECONFIG; export_artifacts "gardener-local2-ha-single-zone" )
  ( make kind-ha-single-zone-down )
  ( make kind2-ha-single-zone-down )
" EXIT

make gardener-ha-single-zone-up
make gardenlet-kind2-ha-single-zone-up
make test-e2e-local-migration-ha-single-zone
make gardener-ha-single-zone-down
make gardenlet-kind2-ha-single-zone-down

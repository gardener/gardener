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
make kind-up
make kind2-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$GARDENER_LOCAL_KUBECONFIG; export_artifacts "gardener-local" )
  ( export KUBECONFIG=$GARDENER_LOCAL2_KUBECONFIG; export_artifacts "gardener-local2" )
  ( make kind-down )
  ( make kind2-down )
" EXIT

make gardener-up
make gardenlet-kind2-up
make test-e2e-local-migration
make gardener-down
make gardenlet-kind2-down

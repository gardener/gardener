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

# If running in prow, we need to ensure that garden.local.gardener.cloud resolves to localhost
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    printf "\n127.0.0.1 garden.local.gardener.cloud\n" >> /etc/hosts
fi

# test setup
make kind-operator-up

# export all container logs and events after test execution
trap "
  ( export KUBECONFIG=$PWD/example/gardener-local/kind/operator/kubeconfig; export_artifacts 'gardener-operator-local'; export_resource_yamls_for garden)
  ( export KUBECONFIG=$PWD/example/operator/virtual-garden/kubeconfig; export_resource_yamls_for seeds shoots; export_events_for_shoots)
  ( make kind-operator-down )
" EXIT

make operator-up
make test-e2e-local-operator-seed
make operator-seed-down
make operator-down

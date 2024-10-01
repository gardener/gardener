#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit
set -x

source $(dirname "${0}")/ci-common.sh

# If running in prow, we need to ensure that garden.local.gardener.cloud resolves to localhost
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    printf "\n127.0.0.1 garden.local.gardener.cloud\n" >> /etc/hosts
    printf "\n::1 garden.local.gardener.cloud\n" >> /etc/hosts
fi

clamp_mss_to_pmtu

# test setup
make kind-ha-multi-zone-up

# export all container logs and events after test execution
trap '{
  export_artifacts "gardener-local-ha-multi-zone"
  make kind-ha-multi-zone-down
}' EXIT

make gardener-ha-multi-zone-up
make test-e2e-local-ha-multi-zone
make gardener-ha-multi-zone-down

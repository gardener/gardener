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

# export all container logs and events after test execution
trap "
  ( export_artifacts "gardener-local" )
  ( make kind-down )
" EXIT

make gardener-up
make test-e2e-local
make gardener-down

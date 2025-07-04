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

make kind-up

trap "
  ( export_artifacts "gardener-local" )
  ( make kind-down )
" EXIT

make gardenadm-up SCENARIO=high-touch
make gardenadm-up SCENARIO=medium-touch

make test-e2e-local-gardenadm

make gardenadm-down SCENARIO=medium-touch
make gardenadm-down SCENARIO=high-touch

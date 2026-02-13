#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

TYPE="default"
case $1 in
  operator|gardenadm)
  TYPE="$1"
  # shift that type argument is removed from ginkgo cli parameters
  shift
  ;;
esac

echo "> E2E Tests"

source "$(dirname "$0")/test-e2e-local.env"
source $(dirname "${0}")/ci-common.sh

ginkgo_flags=

# If running in prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
  mkdir -p "$ARTIFACTS"
  ginkgo_flags="--output-dir=$ARTIFACTS --junit-report=junit.xml"
  if [ "${JOB_TYPE:-}" != "periodic" ]; then
    ginkgo_flags+=" --fail-fast"
  fi
fi

local_address="172.18.255.1"
if [[ "${IPFAMILY:-}" == "ipv6" ]]; then
  local_address="::1"
fi
local_address_operator="172.18.255.3"
if [[ "${IPFAMILY:-}" == "ipv6" ]]; then
  local_address_operator="::3"
fi

GO111MODULE=on ginkgo run --timeout=105m $ginkgo_flags --v --show-node-events "$@"

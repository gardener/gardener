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

ensure_local_gardener_cloud_hosts

# If we are running the gardener-operator tests then we have to make the virtual garden domains accessible.
case $TYPE in
  operator)
    if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
      printf "\n$local_address_operator api.virtual-garden.local.gardener.cloud\n" >>/etc/hosts
      printf "\n$local_address_operator plutono-garden.ingress.runtime-garden.local.gardener.cloud\n" >>/etc/hosts
    else
      if ! grep -q -x "$local_address_operator api.virtual-garden.local.gardener.cloud" /etc/hosts; then
        printf "Hostname for the virtual garden cluster is missing in /etc/hosts. To access the virtual garden cluster and run e2e tests, you need to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#alternative-way-to-set-up-garden-and-seed-leveraging-gardener-operator\n\n"
        exit 1
      fi
      if ! grep -q -x "$local_address_operator plutono-garden.ingress.runtime-garden.local.gardener.cloud" /etc/hosts; then
        printf "Hostname for Plutono is missing in /etc/hosts. To access Plutono and run e2e tests, you need to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#alternative-way-to-set-up-garden-and-seed-leveraging-gardener-operator\n\n"
        exit 1
      fi
    fi
    ;;

  default)
    seed_name="local"
    
    ingress_names=(
      gu-local--e2e-rotate
      gu-local--e2e-rotate-wl
      gu-local--e2e-rot-noroll
      gu-local--e2e-rot-ip
    )

    if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
      for ingress in "${ingress_names[@]}" ; do
        printf "\n$local_address %s.ingress.$seed_name.seed.local.gardener.cloud\n" $ingress >>/etc/hosts
      done
    else
      missing_entries=()

      for ingress in "${ingress_names[@]}" ; do
          if ! grep -q -x "$local_address $ingress.ingress.$seed_name.seed.local.gardener.cloud" /etc/hosts; then
            missing_entries+=("$local_address $ingress.ingress.$seed_name.seed.local.gardener.cloud")
          fi
      done

      if [ ${#missing_entries[@]} -gt 0 ]; then
        printf "Hostnames for the following Shoots are missing in /etc/hosts:\n"
        for entry in "${missing_entries[@]}"; do
          printf " - %s\n" "$entry"
        done
        printf "To access shoot clusters and run e2e tests, you have to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#accessing-the-shoot-cluster\n\n"
        exit 1
      fi
    fi
    ;;
esac

# enable netdns debug log
GODEBUG=netdns=2 \
GO111MODULE=on \
ginkgo run --timeout=105m $ginkgo_flags --v --show-node-events "$@"

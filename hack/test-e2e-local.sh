#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

TYPE="default"
if [[ "$1" == "operator" ]] || [[ "$1" == "operator-seed" ]]; then
  TYPE="$1"
  # shift that "operator" flag is removed from ginkgo cli parameters
  shift
fi

echo "> E2E Tests"

source "$(dirname "$0")/test-e2e-local.env"

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

local_address="127.0.0.1"
if [[ "${IPFAMILY:-}" == "ipv6" ]]; then
  local_address="::1"
fi
local_address_operator="127.0.0.3"
if [[ "${IPFAMILY:-}" == "ipv6" ]]; then
  local_address_operator="::3"
fi

# If running in prow, we need to ensure that garden.local.gardener.cloud resolves to localhost
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    printf "\n127.0.0.1 garden.local.gardener.cloud\n" >> /etc/hosts
fi

# If we are running the gardener-operator tests then we have to make the virtual garden domains accessible.
if [[ "$TYPE" == "operator" ]] || [[ "$TYPE" == "operator-seed" ]]; then
  if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    printf "\n$local_address_operator api.virtual-garden.local.gardener.cloud\n" >>/etc/hosts
    printf "\n$local_address_operator plutono-garden.ingress.runtime-garden.local.gardener.cloud\n" >>/etc/hosts
  else
    if ! grep -q -x "$local_address_operator api.virtual-garden.local.gardener.cloud" /etc/hosts; then
      printf "Hostname for the virtual garden cluster is missing in /etc/hosts. To access the virtual garden cluster and run e2e tests, you need to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#accessing-the-shoot-cluster\n\n"
      exit 1
    fi
    if ! grep -q -x "$local_address_operator plutono-garden.ingress.runtime-garden.local.gardener.cloud" /etc/hosts; then
      printf "Hostname for the plutono is missing in /etc/hosts. To access the plutono and run e2e tests, you need to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#accessing-the-shoot-cluster\n\n"
      exit 1
    fi
  fi

  if [[ "$TYPE" == "operator-seed" ]]; then
    # /etc/hosts must have been updated before garden can be created (otherwise, we could put this command to the
    # hack/ci-e2e-kind-operator-seed.sh script).
    echo "> Deploying Garden and Soil"
    make operator-seed-up
  fi
# If we are not running the gardener-operator tests then we have to make the shoot domains accessible.
else
  seed_name="local"
  if [[ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "node" ]]; then
    seed_name="local-ha-single-zone"
  elif [[ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "zone" ]]; then
    seed_name="local-ha-multi-zone"
  fi

  shoot_names=(
    e2e-managedseed.garden
    e2e-hib.local
    e2e-hib-wl.local
    e2e-unpriv.local
    e2e-wake-up.local
    e2e-wake-up-wl.local
    e2e-migrate.local
    e2e-migrate-wl.local
    e2e-mgr-hib.local
    e2e-rotate.local
    e2e-rotate-wl.local
    e2e-default.local
    e2e-default-wl.local
    e2e-force-delete.local
    e2e-fd-hib.local
    e2e-upd-node.local
    e2e-upd-node-wl.local
    e2e-upgrade.local
    e2e-upgrade-wl.local
    e2e-upg-ha.local
    e2e-upg-ha-wl.local
    e2e-upg-hib.local
    e2e-upg-hib-wl.local
  )

  if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    for shoot in "${shoot_names[@]}" ; do
      if [[ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "zone" && ("$shoot" == "e2e-upg-ha.local" || "$shoot" == "e2e-upg-ha-wl.local") ]]; then
        # Do not add the entry for the e2e-upd-zone test as the target ip is dynamic.
        # The shoot cluster in e2e-upd-zone is created as single-zone control plane and afterwards updated to a multi-zone control plane.
        # This means that the external loadbalancer IP will change from a zone-specific istio ingress gateway to the default istio ingress gateway.
        # A static mapping (to the default istio ingress gateway) as done here will not work in this scenario.
        # The e2e-upd-zone test uses the in-cluster coredns for name resolution and can therefore resolve the api endpoint.
        continue
      fi
      printf "\n$local_address api.%s.external.local.gardener.cloud\n$local_address api.%s.internal.local.gardener.cloud\n" $shoot $shoot >>/etc/hosts
    done
    printf "\n$local_address gu-local--e2e-rotate.ingress.$seed_name.seed.local.gardener.cloud\n" >>/etc/hosts
    printf "\n$local_address gu-local--e2e-rotate-wl.ingress.$seed_name.seed.local.gardener.cloud\n" >>/etc/hosts
  else
    missing_entries=()

    for shoot in "${shoot_names[@]}"; do
      if [[ ("${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "zone" || -z "${SHOOT_FAILURE_TOLERANCE_TYPE:-}") && ("$shoot" == "e2e-upg-ha.local" || "$shoot" == "e2e-upg-ha-wl.local") ]]; then
        # Do not check the entry for the e2e-upg-ha and e2e-upg-ha-wl tests as the target IP is dynamic.
        continue
      fi
      for ip in internal external; do
        if ! grep -q -x "$local_address api.$shoot.$ip.local.gardener.cloud" /etc/hosts; then
          missing_entries+=("$local_address api.$shoot.$ip.local.gardener.cloud")
        fi
      done
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
fi

GO111MODULE=on ginkgo run --timeout=1h $ginkgo_flags --v --show-node-events "$@"

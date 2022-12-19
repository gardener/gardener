#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> E2E Tests"

source "$(dirname "$0")/test-e2e-local.env"

ginkgo_flags=

# If running in prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
  mkdir -p "$ARTIFACTS"
  ginkgo_flags="--output-dir=$ARTIFACTS --junit-report=junit.xml"
fi

# If we are not running the gardener-operator tests then we have to make the shoot domains accessible.
if [[ "$1" != "operator" ]]; then
  seed_name="local";
  if [[ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "node" ]] ; then
    seed_name="local-ha-single-zone";
  fi

  if [[ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" == "zone" ]] ; then
    seed_name="local-ha-multi-zone";
  fi

  shoot_names=(
    e2e-managedseed.garden
    e2e-hibernated.local
    e2e-unpriv.local
    e2e-wake-up.local
    e2e-migrate.local
    e2e-rotate.local
    e2e-default.local
    e2e-update-node.local
    e2e-update-zone.local
    e2e-upgrade.local
  )

  if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    for shoot in "${shoot_names[@]}" ; do
      if [ "${SHOOT_FAILURE_TOLERANCE_TYPE:-}" = "zone" -a "$shoot" = "e2e-update-zone.local" ]; then
        # Do not add the entry for the e2e-update-zone test as the target ip is dynamic.
        # The shoot cluster in e2e-update-zone is created as single-zone control plane and afterwards updated to a multi-zone control plane.
        # This means that the external loadbalancer IP will change from a zone-specific istio ingress gateway to the default istio ingress gateway.
        # A static mapping (to the default istio ingress gateway) as done here will not work in this scenario.
        # The e2e-update-zone test uses the in-cluster coredns for name resolution and can therefore resolve the api endpoint.
        continue
      fi
      printf "\n127.0.0.1 api.%s.external.local.gardener.cloud\n127.0.0.1 api.%s.internal.local.gardener.cloud\n" $shoot $shoot >>/etc/hosts
    done
    printf "\n127.0.0.1 gu-local--e2e-rotate.ingress.$seed_name.seed.local.gardener.cloud\n" >>/etc/hosts
    printf "\n127.0.0.1 api.e2e-managedseed.garden.external.local.gardener.cloud\n127.0.0.1 api.e2e-managedseed.garden.internal.local.gardener.cloud\n" >>/etc/hosts
  else
    for shoot in "${shoot_names[@]}" ; do
      for ip in internal external ; do
        if ! grep -q -x "127.0.0.1 api.$shoot.$ip.local.gardener.cloud" /etc/hosts; then
          printf "Hostnames for Shoot $shoot is missing in /etc/hosts. To access shoot clusters and run e2e tests, you have to extend your /etc/hosts file.\nPlease refer to https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#accessing-the-shoot-cluster\n\n"
          exit 1
        fi
      done
    done
  fi
fi

GO111MODULE=on ginkgo run --timeout=1h $ginkgo_flags --v --progress "$@"

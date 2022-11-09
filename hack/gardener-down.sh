#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SKAFFOLD_PROFILE=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --skaffold-profile)
      shift; SKAFFOLD_PROFILE="-p $1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
kubectl delete seed \
  local \
  local2 \
  local-ha-single-zone \
  local-ha-multi-zone \
  --ignore-not-found \
  --wait \
  --timeout 5m

skaffold delete -m provider-local,gardenlet $SKAFFOLD_PROFILE

kubectl delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
kubectl annotate project local garden confirmation.gardener.cloud/deletion=true

skaffold delete -m local-env $SKAFFOLD_PROFILE
skaffold delete -m etcd,controlplane $SKAFFOLD_PROFILE

# workaround for https://github.com/gardener/gardener/issues/5164
kubectl delete ns \
  seed-local \
  seed-local2 \
  seed-local-ha-single-zone \
  seed-local-ha-multi-zone \
  --ignore-not-found

# cleanup namespaces that don't get deleted automatically
kubectl delete ns gardener-system-seed-lease --ignore-not-found

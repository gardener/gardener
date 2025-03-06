#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

PATH_KIND_KUBECONFIG=""
PATH_GARDEN_KUBECONFIG=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --path-kind-kubeconfig)
      shift; PATH_KIND_KUBECONFIG="$1"
      ;;
    --path-garden-kubeconfig)
      shift; PATH_GARDEN_KUBECONFIG="$1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" delete \
  gardenlet/local \
  seed/local \
  --ignore-not-found \
  --wait \
  --timeout 5m

kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" annotate project local garden confirmation.gardener.cloud/deletion=true
skaffold -f=skaffold.yaml --kubeconfig "$PATH_KIND_KUBECONFIG" delete -m gardenlet -p operator

# workaround for https://github.com/gardener/gardener/issues/5164
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" delete ns \
  seed-local \
  --ignore-not-found

# cleanup garden
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" annotate garden local confirmation.gardener.cloud/deletion=true
skaffold -f=skaffold-operator-garden.yaml --kubeconfig "$PATH_KIND_KUBECONFIG" delete -m garden
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" delete secrets -n garden virtual-garden-etcd-main-backup-local
# delete provider-local extension
kubectl wait --for=condition=RequiredRuntime="False" extensions.operator.gardener.cloud provider-local --timeout=120s
kubectl wait --for=condition=RequiredVirtual="False" extensions.operator.gardener.cloud provider-local --timeout=10s
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" delete extensions.operator.gardener.cloud provider-local

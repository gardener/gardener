#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" annotate project local garden confirmation.gardener.cloud/deletion=true

# cleanup gardenlet and seed
"$(dirname "$0")/operator-gardenlet-down.sh" \
  --path-kind-kubeconfig "$PATH_KIND_KUBECONFIG" \
  --path-garden-kubeconfig "$PATH_GARDEN_KUBECONFIG"

# cleanup garden
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" annotate garden local confirmation.gardener.cloud/deletion=true
skaffold --kubeconfig "$PATH_KIND_KUBECONFIG" delete -m garden
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" delete secrets -n garden virtual-garden-etcd-main-backup-local
# wait for garden deletion
kubectl wait --for=delete garden local --timeout=300s
# wait for provider-local to get uninstalled
kubectl wait --for=condition=RequiredRuntime="False" extensions.operator.gardener.cloud provider-local --timeout=120s
kubectl wait --for=condition=RequiredVirtual="False" extensions.operator.gardener.cloud provider-local --timeout=30s
kubectl wait --for=condition=Installed="False"       extensions.operator.gardener.cloud provider-local --timeout=30s

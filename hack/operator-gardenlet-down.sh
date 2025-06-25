#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

PATH_KIND_KUBECONFIG=""
PATH_GARDEN_KUBECONFIG=""
GARDENLET_NAME="${GARDENLET_NAME:-local}"

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

skaffold --kubeconfig "$PATH_GARDEN_KUBECONFIG" delete -m gardenlet
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" delete seed/"$GARDENLET_NAME" --ignore-not-found --wait --timeout 5m
kubectl --kubeconfig "$PATH_KIND_KUBECONFIG" -n garden delete deployment gardenlet --ignore-not-found

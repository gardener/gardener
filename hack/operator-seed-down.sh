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

# cleanup gardenlet and seed
"$(dirname "$0")/operator-gardenlet-down.sh" \
  --path-kind-kubeconfig "$PATH_KIND_KUBECONFIG" \
  --path-garden-kubeconfig "$PATH_GARDEN_KUBECONFIG"

# cleanup garden
"$(dirname "$0")/../dev-setup/garden.sh" down

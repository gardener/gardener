#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

source "$(dirname "$0")/scenario.sh"

detect_scenario
skaffold_profile "$SCENARIO"

COMMAND="${1:-up}"
VALID_COMMANDS=("up dev debug down")

skaffold_command="run"
if [[ "$COMMAND" == "dev" ]]; then
  skaffold_command="dev"
elif [[ "$COMMAND" == "debug" ]]; then
  skaffold_command="debug"
fi

gardenlet_name="local"
if [[ "$SKAFFOLD_PROFILE" == "multi-node2" ]]; then
  gardenlet_name="local2"
fi

case "$COMMAND" in
  up)
    skaffold run \
      -m garden-config \
      --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" \
		  --status-check=false --platform="linux/$SYSTEM_ARCH" 	# deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/

    skaffold $skaffold_command \
      -m gardenlet \
      --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" \
      --cache-artifacts="$($(dirname "$0")/../hack/get-skaffold-cache-artifacts.sh)" \
      --status-check=false --platform="linux/$SYSTEM_ARCH" # deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/
    ;;

  down)
    skaffold --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" delete -m gardenlet
    kubectl  --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" delete seed/"$gardenlet_name" --ignore-not-found --wait --timeout 5m
    kubectl  -n garden delete deployment gardenlet --ignore-not-found
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

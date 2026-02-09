#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

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
elif [[ "$SKAFFOLD_PROFILE" == "remote" ]]; then
  gardenlet_name="remote"
fi

if [[ "$SCENARIO" == "remote" ]]; then
  registry_domain=$(cat "$SCRIPT_DIR/remote/registry/registrydomain")
  export SKAFFOLD_DEFAULT_REPO="$registry_domain"
fi

# We assume that all nodes of the cluster have the same architecture.
SYSTEM_ARCH=$(kubectl get nodes -o yaml | yq '.items[0].status.nodeInfo.architecture')

case "$COMMAND" in
  up)
    skaffold run \
      -v debug \
      -m garden-config \
      --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" \
      --status-check=false --platform="linux/$SYSTEM_ARCH" # deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/

    skaffold $skaffold_command \
      -m gardenlet \
      --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" \
      --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh)" \
      --status-check=false --platform="linux/$SYSTEM_ARCH" # deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/

    if [[ "$SCENARIO" == "remote" ]]; then
      "$SCRIPT_DIR"/remote/registry/deploy-registry.sh "$KUBECONFIG" "$registry_domain" "$VIRTUAL_GARDEN_KUBECONFIG"
      kubectl apply -k "$SCRIPT_DIR/remote/registry/kyverno-policies"
    fi
    ;;

  down)
    skaffold --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" delete -m gardenlet
    kubectl  --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" delete seed/"$gardenlet_name" --ignore-not-found --wait --timeout 5m
    kubectl  -n garden delete deployment gardenlet --ignore-not-found

    if [[ "$SCENARIO" == "remote" ]]; then
      kubectl delete -k "$SCRIPT_DIR/remote/registry/kyverno-policies" --ignore-not-found
    fi
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

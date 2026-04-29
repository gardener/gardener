#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

SCENARIO="${SCENARIO:-unmanaged-infra}"
VALID_SCENARIOS=("unmanaged-infra" "managed-infra" "connect" "connect-kind")

# For unmanaged-infra and connect scenarios, there is no cluster to detect the node platform from.
# Default to the local machine's architecture.
if [[ "$SCENARIO" == "unmanaged-infra" || "$SCENARIO" == "connect" ]]; then
  export SKAFFOLD_PLATFORM="${SKAFFOLD_PLATFORM:-linux/$(go env GOARCH)}"
  export SKAFFOLD_CHECK_CLUSTER_NODE_PLATFORMS=false
fi

valid_scenario=false
for scenario in "${VALID_SCENARIOS[@]}"; do
  if [[ "$SCENARIO" == "$scenario" ]]; then
    valid_scenario=true
    break
  fi
done
if ! $valid_scenario; then
  echo "Error: Invalid scenario '${SCENARIO}'. Valid options are: ${VALID_SCENARIOS[*]}." >&2
  exit 1
fi

garden_runtime_cluster_kubeconfig="$KUBECONFIG_RUNTIME_CLUSTER"
if [[ "$SCENARIO" == "connect" ]]; then
  garden_runtime_cluster_kubeconfig="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
  ./hack/usage/generate-kubeconfig.sh self-hosted-shoot --docker gind-machine-0 > "$garden_runtime_cluster_kubeconfig"
fi

case "$COMMAND" in
  up)
    if [[ "$SCENARIO" != connect* ]]; then
      # Prepare resources and generate manifests.
      # The manifests are copied to the unmanaged-infra machine pods or can be passed to the `--config-dir` flag of `gardenadm bootstrap`.
      skaffold build \
        -p "$SCENARIO" \
        -m gardenadm,provider-local \
        -q \
        --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh gardenadm)" \
        |\
      skaffold render \
        -p "$SCENARIO" \
        -m provider-local \
        -o "$(dirname "$0")/gardenadm/resources/generated/$SCENARIO/manifests.yaml" \
        --build-artifacts \
        -

      # Export global resources for `gardenadm connect` scenario in case they will be needed later
      mkdir -p "$(dirname "$0")/gardenadm/resources/generated/connect"
      # We don't need to export Controller{Registration,Deployment}s since they already get registered by
      # gardener-operator.
      yq '. | select(.kind == "Project" or .kind == "Namespace" or .kind == "CloudProfile")' \
        < "$(dirname "$0")/gardenadm/resources/generated/$SCENARIO/manifests.yaml" \
        > "$(dirname "$0")/gardenadm/resources/generated/connect/manifests.yaml"
    else
      if [[ ! -f "$(dirname "$0")/gardenadm/resources/generated/connect/manifests.yaml" ]]; then
        echo "Error: Must run 'make gardenadm-up' first." >&2
        exit 1
      fi

      make operator-up garden-up \
        -f "$(dirname "$0")/../Makefile" \
        KUBECONFIG="$garden_runtime_cluster_kubeconfig"

      echo "Creating global resources in the virtual garden cluster as preparation for running 'gardenadm connect'..."
      kubectl --kubeconfig="$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" apply -f "$(dirname "$0")/gardenadm/resources/generated/connect/manifests.yaml"
    fi
    ;;

  down)
    if [[ "$SCENARIO" != "connect" ]]; then
      skaffold delete \
        -n "gardenadm-$SCENARIO"
    else
      make garden-down operator-down \
        -f "$(dirname "$0")/../Makefile" \
        KUBECONFIG="$garden_runtime_cluster_kubeconfig"
    fi
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

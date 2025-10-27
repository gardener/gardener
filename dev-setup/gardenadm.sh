#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

SCENARIO="${SCENARIO:-unmanaged-infra}"
VALID_SCENARIOS=("unmanaged-infra" "managed-infra" "connect")

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

case "$COMMAND" in
  up)
    if [[ "$SCENARIO" != "connect" ]]; then
      # Prepare resources and generate manifests.
      # The manifests are copied to the unmanaged-infra machine pods or can be passed to the `--config-dir` flag of `gardenadm bootstrap`.
      skaffold build \
        -p "$SCENARIO" \
        -m gardenadm,provider-local-node,provider-local \
        -q \
        --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh gardenadm)" \
        |\
      skaffold render \
        -p "$SCENARIO" \
        -m provider-local-node,provider-local \
        -o "$(dirname "$0")/gardenadm/resources/generated/$SCENARIO/manifests.yaml" \
        --build-artifacts \
        -

      if [[ "$SCENARIO" == "unmanaged-infra" ]]; then
        skaffold run \
          -n gardenadm-unmanaged-infra \
          -m provider-local-node,machine
      fi

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
        KUBECONFIG="$KUBECONFIG"

      echo "Creating global resources in the virtual garden cluster as preparation for running 'gardenadm connect'..."
      kubectl --kubeconfig="$VIRTUAL_GARDEN_KUBECONFIG" apply -f "$(dirname "$0")/gardenadm/resources/generated/connect/manifests.yaml"
    fi
    ;;

  down)
    if [[ "$SCENARIO" != "connect" ]]; then
      skaffold delete \
        -n "gardenadm-$SCENARIO"
    else
      make garden-down operator-down \
        -f "$(dirname "$0")/../Makefile" \
        KUBECONFIG="$KUBECONFIG"
    fi
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

SCENARIO="${SCENARIO:-default}"
declare -A SCENARIO_LEVEL=(
  [default]=1  # Run `gardenadm bootstrap` and export the kubeconfig for the self-hosted shoot
  [connect]=2  # Like 'default', but also deploys Gardener into the self-hosted shoot and runs `gardenadm connect` to deploy gardenlet which registers the Shoot
  [full]=3     # Like 'connect', but also registers the self-hosted shoot as a seed via a ManagedSeed
)

if [[ -z "${SCENARIO_LEVEL[$SCENARIO]+x}" ]]; then
  echo "Error: Invalid scenario '${SCENARIO}'. Valid options are: ${!SCENARIO_LEVEL[*]}." >&2
  exit 1
fi

level="${SCENARIO_LEVEL[$SCENARIO]}"

GARDENADM_RESOURCES_DIR="$(dirname "$0")/gardenadm/resources"
GARDENADM_GENERATED_DIR="$GARDENADM_RESOURCES_DIR/generated"

case "$COMMAND" in
  up)
    make kind-up

    make gardenadm-up SCENARIO=managed-infra
    make gardenadm

    GARDENADM_BOOTSTRAP_FLAGS="${GARDENADM_BOOTSTRAP_FLAGS:-}"

    KUBECONFIG="$KUBECONFIG_RUNTIME_CLUSTER" \
    IMAGEVECTOR_OVERWRITE="$GARDENADM_GENERATED_DIR/.imagevector-overwrite.yaml" \
    IMAGEVECTOR_OVERWRITE_COMPONENTS="$GARDENADM_RESOURCES_DIR/imagevector-overwrite-components.yaml" \
    IMAGEVECTOR_OVERWRITE_CHARTS="$GARDENADM_GENERATED_DIR/.imagevector-overwrite-charts.yaml" \
      "$(dirname "$0")/../bin/gardenadm" bootstrap \
        -d "$GARDENADM_GENERATED_DIR/managed-infra" \
        --kubeconfig-output "$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER" \
        ${GARDENADM_BOOTSTRAP_FLAGS}

    cp "$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER" "$(dirname "$0")/gardenconfig/components/credentials/secret-project-garden/kubeconfig/kubeconfig"
    cp "$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER" "$(dirname "$0")/gardenconfig/components/credentials/secret-project-local/kubeconfig/kubeconfig"

    kubectl --kubeconfig "$KUBECONFIG_RUNTIME_CLUSTER" scale deployment gardener-resource-manager -n shoot--garden--root --replicas=0

    # Deploy Gardener into the self-hosted shoot and run `gardenadm connect` to deploy gardenlet which registers the Shoot
    if (( level >= 2 )); then
      make gardenadm-up SCENARIO=connect-managed # deploys gardener-operator, the 'Garden' resource, and waits for reconciliation
      connect_command="$(KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER "$(dirname "$0")/../bin/gardenadm" token create --print-connect-command --shoot-namespace garden --shoot-name root)"
      # The connect command must run inside the control plane machine pod (mirroring how gind.sh runs it in machine-0
      # via docker exec). The machine pod has gardenadm installed in /gardenadm/gardenadm.
      technical_id="shoot--garden--root"
      machine_namespace="infra-${technical_id}"
      machine_pod="$(kubectl --kubeconfig "$KUBECONFIG_RUNTIME_CLUSTER" -n "$machine_namespace" get pods -l app=machine --sort-by=.metadata.name -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep "${technical_id}-control-plane-" | head -1)"
      kubectl --kubeconfig "$KUBECONFIG_RUNTIME_CLUSTER" exec -n "$machine_namespace" "$machine_pod" -c node -- bash -c "IMAGEVECTOR_OVERWRITE=/var/lib/gardenadm/imagevector-overwrite.yaml IMAGEVECTOR_OVERWRITE_CHARTS=/var/lib/gardenadm/imagevector-overwrite-charts.yaml /opt/bin/${connect_command}"
    fi

    # Register the self-hosted shoot as a seed via a ManagedSeed
    if (( level >= 3 )); then
      "$(dirname "$0")/gardenlet/overlays/multi-node-gardenadm/generate-patch-managedseed.sh" managed-infra
      make seed-up KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi
    ;;

  down)
    if kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" -n garden get managedseed root &>/dev/null; then
      make seed-down KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi

    make gardenadm-down SCENARIO=managed-infra

    make kind-down
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

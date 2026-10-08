#!/usr/bin/env bash
# SPDX-FileCopyrightText: Contributors to the Gardener project
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

    # Deploy Gardener into the self-hosted shoot and run `gardenadm connect` to deploy gardenlet which registers the Shoot
    if (( level >= 2 )); then
      make gardenadm-up SCENARIO=connect # deploys gardener-operator, the 'Garden' resource, and waits for reconciliation
      connect_command="$(KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER "$(dirname "$0")/../bin/gardenadm" token create --print-connect-command --shoot-namespace garden --shoot-name root)"
      # In contrast to gind.sh (which runs the connect command inside a machine), run it from the host against the
      # self-hosted shoot's API server, using the same resources as for `gardenadm bootstrap`.
      # The generated command starts with 'gardenadm', so we prefix it with the path to the locally built binary.
      KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER" \
      IMAGEVECTOR_OVERWRITE="$GARDENADM_GENERATED_DIR/.imagevector-overwrite.yaml" \
      IMAGEVECTOR_OVERWRITE_COMPONENTS="$GARDENADM_RESOURCES_DIR/imagevector-overwrite-components.yaml" \
      IMAGEVECTOR_OVERWRITE_CHARTS="$GARDENADM_GENERATED_DIR/.imagevector-overwrite-charts.yaml" \
        bash -c "$(dirname "$0")/../bin/${connect_command} -d $GARDENADM_GENERATED_DIR/managed-infra"
    fi

    # Register the self-hosted shoot as a seed via a ManagedSeed
    if (( level >= 3 )); then
      make seed-up KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi
    ;;

  down)
    if kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" --request-timeout 1s -n garden get managedseed root &>/dev/null; then
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

#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

GIND_COMPOSE_FILE="$(dirname "$0")/gind/docker-compose.yaml"

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

SCENARIO="${SCENARIO:-default}"
declare -A SCENARIO_LEVEL=(
  [machines]=1 # Only start gind machine containers and installs gardenadm, but doesn't run it
  [default]=2  # Like 'machines', but also runs `gardenadm init` and exports the kubeconfig for the self-hosted shoot
  [join]=3     # Like 'default', but also runs `gardenadm join` on gind-machine-1 to join it as worker node
  [connect]=4  # Like 'join', but also deploys Gardener into the self-hosted shoot and runs `gardenadm connect` to deploy gardenlet which registers the Shoot
  [full]=5     # Like 'connect', but also registers the self-hosted shoot as a seed via a ManagedSeed
)

if [[ -z "${SCENARIO_LEVEL[$SCENARIO]+x}" ]]; then
  echo "Error: Invalid scenario '${SCENARIO}'. Valid options are: ${!SCENARIO_LEVEL[*]}." >&2
  exit 1
fi

level="${SCENARIO_LEVEL[$SCENARIO]}"

case "$COMMAND" in
  up)
    "$(dirname "$0")/infra.sh" up

    # Compute a checksum-based image tag so that `docker compose up` only rebuilds and recreates the machine containers
    # when the Dockerfile or its build context actually changed.
    GIND_BUILD_CONTEXT="$(dirname "$0")/../pkg/provider-local/machine-provider/node"
    export GIND_MACHINE_IMAGE="gind-machine:$(find "$GIND_BUILD_CONTEXT" -type f | sort | xargs shasum -a 256 | shasum -a 256 | cut -c1-12)"
    docker compose -f "$GIND_COMPOSE_FILE" up -d

    make gardenadm-up SCENARIO=unmanaged-infra

    for i in 0 1 2 3; do
      service="machine-$i"
      docker compose -f "$GIND_COMPOSE_FILE" exec "$service" bash -c 'mkdir -p /gardenadm/resources'
      docker compose -f "$GIND_COMPOSE_FILE" cp "$(dirname "$0")/gardenadm/resources/generated/.skaffold-image"                    "$service:/gardenadm/.skaffold-image"
      docker compose -f "$GIND_COMPOSE_FILE" cp "$(dirname "$0")/gardenadm/resources/generated/.imagevector-overwrite.yaml"        "$service:/gardenadm/imagevector-overwrite.yaml"
      docker compose -f "$GIND_COMPOSE_FILE" cp "$(dirname "$0")/gardenadm/resources/generated/.imagevector-overwrite-charts.yaml" "$service:/gardenadm/imagevector-overwrite-charts.yaml"
      docker compose -f "$GIND_COMPOSE_FILE" cp "$(dirname "$0")/gardenadm/resources/generated/unmanaged-infra/manifests.yaml"     "$service:/gardenadm/resources/manifests.yaml"

      docker compose -f "$GIND_COMPOSE_FILE" cp "$(dirname "$0")/gind/install-gardenadm.sh" "$service:/install-gardenadm.sh"
      docker compose -f "$GIND_COMPOSE_FILE" exec "$service" bash -c '/install-gardenadm.sh $(cat /gardenadm/.skaffold-image)'
    done

    # Run `gardenadm init` and export the kubeconfig for the self-hosted shoot
    if (( level >= 2 )); then
      if [[ "${FAST:-}" == "true" ]]; then
        GARDENADM_INIT_FLAGS="${GARDENADM_INIT_FLAGS:-} --use-bootstrap-etcd --use-host-network"
      fi
      docker compose -f "$GIND_COMPOSE_FILE" exec machine-0 bash -c "gardenadm init -d /gardenadm/resources ${GARDENADM_INIT_FLAGS:-}"
      ./hack/usage/generate-kubeconfig.sh self-hosted-shoot --docker gind-machine-0 > "$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi

    # Run `gardenadm join` on gind-machine-1 to join it as worker node
    if (( level >= 3 )); then
      join_command="$(docker compose -f "$GIND_COMPOSE_FILE" exec machine-0 bash -c 'gardenadm token create --print-join-command')"
      docker compose -f "$GIND_COMPOSE_FILE" exec machine-1 bash -c "$join_command"
    fi

    # Deploy Gardener into the self-hosted shoot and run `gardenadm connect` to deploy gardenlet which registers the Shoot
    if (( level >= 4 )); then
      make gardenadm-up SCENARIO=connect # deploys gardener-operator, the 'Garden' resource, and waits for reconciliation
      make gardenadm # builds gardenadm binary locally so that we can execute it against the virtual-garden-cluster
      connect_command="$(KUBECONFIG=$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER "$(dirname "$0")/../bin/gardenadm" token create --print-connect-command --shoot-namespace garden --shoot-name root)"
      docker compose -f "$GIND_COMPOSE_FILE" exec machine-0 bash -c "$connect_command"
    fi

    # Register the self-hosted shoot as a seed via a ManagedSeed
    if (( level >= 5 )); then
      make seed-up KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi
    ;;

  down)
    if kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" -n garden get managedseed root &>/dev/null; then
      make seed-down KUBECONFIG="$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    fi

    make gardenadm-down SCENARIO=unmanaged-infra

    docker compose -f "$GIND_COMPOSE_FILE" down --volumes

    "$(dirname "$0")/infra.sh" down
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

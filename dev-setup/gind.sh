#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

GIND_COMPOSE_FILE="$(dirname "$0")/gind/docker-compose.yaml"

case "$COMMAND" in
  up)
    "$(dirname "$0")/infra.sh" up

    # Compute a checksum-based image tag so that `docker compose up` only rebuilds and recreates the machine containers
    # when the Dockerfile or its build context actually changed.
    GIND_BUILD_CONTEXT="$(dirname "$0")/../pkg/provider-local/machine-provider/node"
    export GIND_MACHINE_IMAGE="gind-machine:$(find "$GIND_BUILD_CONTEXT" -type f | sort | xargs shasum -a 256 | shasum -a 256 | cut -c1-12)"
    docker compose -f "$GIND_COMPOSE_FILE" up -d

    make gardenadm-up SKAFFOLD_PLATFORM="linux/$(go env GOARCH)" SKAFFOLD_CHECK_CLUSTER_NODE_PLATFORMS=false

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

    docker compose -f "$GIND_COMPOSE_FILE" exec machine-0 bash -c 'gardenadm init -d /gardenadm/resources'

    ./hack/usage/generate-kubeconfig.sh self-hosted-shoot --docker gind-machine-0 > "$KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER"
    ;;

  down)
    make gardenadm-down SKAFFOLD_PLATFORM="linux/$(go env GOARCH)" SKAFFOLD_CHECK_CLUSTER_NODE_PLATFORMS=false

    docker compose -f "$GIND_COMPOSE_FILE" down --volumes

    "$(dirname "$0")/infra.sh" down
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

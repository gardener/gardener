#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

source "$(dirname "$0")/scenario.sh"

detect_scenario

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "dev" "debug" "down")

if [[ "$SCENARIO" == "remote" ]]; then
  registry_domain=$(cat "$SCRIPT_DIR/remote/registry/registrydomain")
  export SKAFFOLD_DEFAULT_REPO="$registry_domain"
fi

case "$COMMAND" in
  up)
    skaffold run \
      --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh)"
   ;;

  dev)
    skaffold dev
   ;;

  debug)
    skaffold debug
   ;;

  down)
    kubectl annotate garden --all confirmation.gardener.cloud/deletion=true
    kubectl delete   garden --all --ignore-not-found --wait --timeout 5m
    skaffold delete
    # Webhook configurations have to be deleted manually
    kubectl delete mutatingwebhookconfigurations gardener-operator --ignore-not-found
    kubectl delete validatingwebhookconfigurations gardener-operator --ignore-not-found
    # Remove finalizers from extensions.operator.gardener.cloud until gardener-operator handles their removal properly
    for name in $(kubectl get extensions.operator.gardener.cloud -o jsonpath='{.items[*].metadata.name}'); do
      kubectl patch extensions.operator.gardener.cloud "$name" --type=json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' || true
    done
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

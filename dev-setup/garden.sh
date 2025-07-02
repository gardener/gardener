#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

function detect_scenario() {
  nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.labels.topology\.kubernetes\.io/zone}' | tr ' ' '\n' | sort -u)
  if [[ $(echo "$nodes" | wc -l) -eq 1 ]]; then
    echo "multi-node"
  elif [[ $(echo "$nodes" | wc -l) -eq 3 ]]; then
    echo "multi-zone"
  else
    return 1
  fi
}

SCENARIO="$(detect_scenario)"
if [[ -z "$SCENARIO" ]]; then
  echo "Error: Unable to detect scenario. Please ensure you have a valid Kubernetes cluster with correctly labeled nodes with their availability zone." >&2
  exit 1
fi
echo "DETECTED SCENARIO: $SCENARIO"

COMMAND="${1:-up}"
VALID_COMMANDS=("up down")

case "$COMMAND" in
  up)
    kubectl apply -k "$(dirname "$0")/garden/overlays/$SCENARIO"
    # We deliberately only wait for the last operation to be 'Reconcile Succeeded' in order to be able to faster deploy
    # the gardenlet.
    TIMEOUT=900 "$(dirname "$0")"/../hack/usage/wait-for.sh garden local
    # Check that the admission component of provider-local extension is healthy - it may run webhooks for the resources
    # that we are about to create in below 'garden-config' Skaffold config. This will fail in case the webhook server is
    # down or not yet available.
    TIMEOUT=60 SKIP_LAST_OPERATION_CHECK=true "$(dirname "$0")"/../hack/usage/wait-for.sh extop provider-local AdmissionHealthy
    # Export kubeconfig for the virtual garden cluster
    "$(dirname "$0")"/../hack/usage/generate-virtual-garden-admin-kubeconf.sh > "$VIRTUAL_GARDEN_KUBECONFIG"
    ;;

  down)
    kubectl --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" annotate projects local garden confirmation.gardener.cloud/deletion=true || true
    kubectl annotate garden local confirmation.gardener.cloud/deletion=true || true

    kubectl delete -k "$(dirname "$0")/garden/overlays/$SCENARIO" --ignore-not-found

    echo "Waiting for the garden to be deleted..."
    kubectl wait --for=delete                            garden                             local          --timeout=300s
    echo "Waiting for the provider-local extension to be uninstalled..."
    kubectl wait --for=condition=RequiredRuntime="False" extensions.operator.gardener.cloud provider-local --timeout=120s
    kubectl wait --for=condition=RequiredVirtual="False" extensions.operator.gardener.cloud provider-local --timeout=30s
    kubectl wait --for=condition=Installed="False"       extensions.operator.gardener.cloud provider-local --timeout=30s
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

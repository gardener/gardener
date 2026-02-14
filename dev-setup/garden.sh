#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

source "$(dirname "$0")/scenario.sh"

detect_scenario

COMMAND="${1:-up}"
VALID_COMMANDS=("up down")

garden_name="local"
if [[ "$SCENARIO" == "remote" ]]; then
  garden_name="remote"
fi

case "$COMMAND" in
  up)
    echo "Waiting for Garden CRD to be established..."
    kubectl wait --for=condition=Established crd/gardens.operator.gardener.cloud --timeout=60s

    kubectl apply -k "$(dirname "$0")/garden/overlays/$SCENARIO"
    # We deliberately only wait for the last operation to be 'Reconcile Succeeded' in order to be able to faster deploy
    # the gardenlet.
    TIMEOUT=900 "$(dirname "$0")"/../hack/usage/wait-for.sh garden $garden_name
    if [[ "$SCENARIO" != "remote" ]]; then
      # Check that the admission component of provider-local extension is healthy - it may run webhooks for the resources
      # that we are about to create in below 'garden-config' Skaffold config. This will fail in case the webhook server is
      # down or not yet available.
      TIMEOUT=60 SKIP_LAST_OPERATION_CHECK=true "$(dirname "$0")"/../hack/usage/wait-for.sh extop provider-local AdmissionHealthy
    fi
    # Export kubeconfig for the virtual garden cluster
    "$(dirname "$0")"/../hack/usage/generate-virtual-garden-admin-kubeconf.sh "$KUBECONFIG" "$garden_name"  > "$VIRTUAL_GARDEN_KUBECONFIG"
    ;;

  down)
    kubectl --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" annotate projects $garden_name garden confirmation.gardener.cloud/deletion=true || true
    kubectl annotate garden $garden_name confirmation.gardener.cloud/deletion=true || true
    kubectl delete garden $garden_name --wait=false --ignore-not-found

    echo "Waiting for the garden to be deleted..."
    kubectl wait --for=delete                            garden                             $garden_name   --timeout=300s
    if [[ "$SCENARIO" != "remote" ]]; then
      echo "Waiting for the provider-local extension to be uninstalled..."
      kubectl wait --for=condition=RequiredRuntime="False" extensions.operator.gardener.cloud provider-local --timeout=120s
      kubectl wait --for=condition=RequiredVirtual="False" extensions.operator.gardener.cloud provider-local --timeout=30s
      kubectl wait --for=condition=Installed="False"       extensions.operator.gardener.cloud provider-local --timeout=30s
    fi

    # cleanup the remaining resources required for successful deletion of the garden
    kubectl delete -k "$(dirname "$0")/garden/overlays/$SCENARIO" --ignore-not-found
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

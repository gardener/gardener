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

    # Check whether the current gardener-operator pod(s) already reconciled (or are currently reconciling) the Garden.
    # If so, we don't need to trigger an explicit reconciliation via the operation annotation. This avoids unnecessary
    # double reconciliations when the operator was just (re)deployed and already picked up the Garden.
    garden_status_gardener_name=$(kubectl get garden "$garden_name" -o jsonpath='{.status.gardener.name}' 2>/dev/null || true)
    garden_last_operation_state=$(kubectl get garden "$garden_name" -o jsonpath='{.status.lastOperation.state}' 2>/dev/null || true)
    operator_pod_names=$(kubectl get pods -n garden -l app=gardener,role=operator --field-selector=status.phase=Running -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)

    needs_reconciliation=true
    if [[ -n "$garden_status_gardener_name" && -n "$operator_pod_names" ]]; then
      for pod_name in $operator_pod_names; do
        if [[ "$garden_status_gardener_name" == "$pod_name" && ("$garden_last_operation_state" == "Succeeded" || "$garden_last_operation_state" == "Processing") ]]; then
          needs_reconciliation=false
          break
        fi
      done
    fi

    if [[ "$needs_reconciliation" == "true" ]]; then
      echo "Triggering Garden reconciliation..."
      kubectl annotate garden "$garden_name" gardener.cloud/operation=reconcile
    else
      echo "Garden is already reconciled by the current gardener-operator ($garden_status_gardener_name), skipping reconcile annotation."
    fi

    # We deliberately only wait for the last operation to be 'Reconcile Succeeded' in order to be able to faster deploy
    # the gardenlet.
    TIMEOUT=900 "$(dirname "$0")"/../hack/usage/wait-for.sh garden "$garden_name"
    if [[ "$SCENARIO" != "remote" ]]; then
      # Check that the admission component of provider-local extension is healthy - it may run webhooks for the resources
      # that we are about to create in below 'garden-config' Skaffold config. This will fail in case the webhook server is
      # down or not yet available.
      TIMEOUT=60 SKIP_LAST_OPERATION_CHECK=true "$(dirname "$0")"/../hack/usage/wait-for.sh extop provider-local AdmissionHealthy
    fi
    # Export kubeconfig for the virtual garden cluster
    RUNTIME_CLUSTER_KUBECONFIG="$KUBECONFIG" GARDEN_NAME="$garden_name" "$(dirname "$0")"/../hack/usage/generate-kubeconfig.sh virtual-garden > "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"
    # Rerun registry script to deploy pull secret into virtual garden
    if [[ "$SCENARIO" == "remote" ]]; then
      registry_domain=$(cat "$(dirname "$0")/remote/registry/registrydomain")
      "$(dirname "$0")"/remote/registry/deploy-registry.sh "$KUBECONFIG" "$registry_domain" "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"
    fi
    ;;

  down)
    kubectl --kubeconfig "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" annotate projects $garden_name garden confirmation.gardener.cloud/deletion=true || true
    kubectl annotate garden $garden_name confirmation.gardener.cloud/deletion=true || true
    kubectl delete garden $garden_name --wait=false --ignore-not-found

    echo "Waiting for the garden to be deleted..."
    kubectl wait --for=delete                            garden                             $garden_name   --timeout=480s
    if [[ "$SCENARIO" != "remote" ]]; then
      echo "Waiting for the provider-local extension to be uninstalled..."
      kubectl wait --for=condition=RequiredRuntime="False" extensions.operator.gardener.cloud provider-local --timeout=120s
      kubectl wait --for=condition=RequiredVirtual="False" extensions.operator.gardener.cloud provider-local --timeout=30s
      kubectl wait --for=condition=Installed="False"       extensions.operator.gardener.cloud provider-local --timeout=30s
    fi

    # cleanup the remaining resources required for successful deletion of the garden
    kubectl delete -k "$(dirname "$0")/garden/overlays/$SCENARIO" --ignore-not-found

    # cleanup all PVCs in the garden namespace
    echo "Cleaning up PVCs in garden namespace..."
    kubectl delete pvc --all -n garden --ignore-not-found --timeout=120s

    # cleanup virtual garden kubconfig
    if [[ -f "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER" ]]; then
      echo "Deleting virtual garden kubeconfig at $KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"
      rm "$KUBECONFIG_VIRTUAL_GARDEN_CLUSTER"
    fi
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac

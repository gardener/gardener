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

garden_runtime_cluster_kubeconfig="$KUBECONFIG"
if [[ "$SCENARIO" == "connect" ]]; then
  garden_runtime_cluster_kubeconfig="$(dirname "$0")/kubeconfigs/self-hosted-shoot/kubeconfig"
  ./hack/usage/generate-kubeconfig.sh self-hosted-shoot > "$garden_runtime_cluster_kubeconfig"
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

      if [[ "$SCENARIO" == "unmanaged-infra" ]]; then
        skaffold run \
          -p "$SCENARIO" \
          -n gardenadm-unmanaged-infra \
          -m provider-local,machine
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

      if [[ "$SCENARIO" == "connect" ]]; then
        # Used to talk to the virtual-garden API server from the host machine via the following network path:
        # Host:172.18.255.3:443
        #   → Docker (hostPort 443 → containerPort 31443)
        #     → KinD node, NodePort 31443 (service gardenadm-unmanaged-infra/control-plane-machine)
        #       → machine-0 pod IP 10.0.212.0:31443
        #         → istio-ingressgateway pod exposed via NodePort 31443 in the machine-0 node (patched by MutatingAdmissionPolicy loadbalancer-services)
        # In the 'connect-kind' scenario, the istio-ingressgateway runs directly on the KinD cluster and gets exposed
        # via a dynamic load balancer provisioned by cloud-controller-manager-local, hence, no need to patch this
        # Service in this scenario.
        kubectl --kubeconfig "$garden_runtime_cluster_kubeconfig" apply -k "$(dirname "$0")/../dev-setup/gardenadm/loadbalancer-services" --server-side
        if ! kubectl --kubeconfig "$KUBECONFIG" -n gardenadm-unmanaged-infra get service control-plane-machine \
          -o jsonpath='{.spec.ports[*].name}' | grep -qw virtual-garden-apiserver; then
          kubectl --kubeconfig "$KUBECONFIG" -n gardenadm-unmanaged-infra patch service control-plane-machine \
            --type=json \
            -p '[{"op":"add","path":"/spec/ports/-","value":{"name":"virtual-garden-apiserver","port":31443,"targetPort":31443,"nodePort":31443}}]'
        fi
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

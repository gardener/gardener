#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

function detect_kind_cluster_name() {
  provider_id=$(kubectl get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[*].spec.providerID}' | tr ' ' '\n' | head -1)

  if ! [[ "$provider_id" == kind://* ]]; then
    echo "Error: Unable to detect kind cluster name. Couldn't find control-plane node with providerID starting with 'kind://'." >&2
    return 1
  fi

  CLUSTER_NAME=${provider_id#kind://docker/}
  CLUSTER_NAME=${CLUSTER_NAME%%/*}
  export CLUSTER_NAME

  echo "Detected cluster name: $CLUSTER_NAME"
}

function detect_scenario() {
  nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n')
  zones=$(kubectl get nodes -o jsonpath='{.items[*].metadata.labels.topology\.kubernetes\.io/zone}' | tr ' ' '\n' | sort -u)
  provider_ids=$(kubectl get nodes -o jsonpath='{.items[*].spec.providerID}' | tr ' ' '\n')

  # Check if all providerIDs have a scheme (contain "://") but none start with kind://
  if [[ -n "$provider_ids" && $(echo "$provider_ids" | grep -c '://') -eq $(echo "$provider_ids" | wc -l) && $(echo "$provider_ids" | grep -cv '^kind://') -eq $(echo "$provider_ids" | wc -l) ]]; then
    export SCENARIO="remote"
  elif [[ $(echo "$nodes" | wc -l) -eq 1 ]]; then
    export SCENARIO="single-node"
  elif [[ $(echo "$zones" | wc -l) -eq 1 ]]; then
    export SCENARIO="multi-node"
  elif [[ $(echo "$zones" | wc -l) -eq 3 ]]; then
    export SCENARIO="multi-zone"
  else
    echo "Error: Unable to detect scenario. Please ensure you have a valid Kubernetes cluster with correctly labeled nodes with their availability zone." >&2
    exit 1
  fi

  if grep -q "gardener-local2" <<< "$nodes"; then
    export SCENARIO="${SCENARIO}2"
  fi

  if [[ "$IPFAMILY" == "ipv6" ]]; then
    export SCENARIO="${SCENARIO}-ipv6"
  elif [[ "$IPFAMILY" == "dual" ]]; then
    export SCENARIO="${SCENARIO}-dual"
  fi

  if [[ "$(kubectl get namespace kube-system -o jsonpath='{.metadata.labels.gardener\.cloud/role}')" == "shoot" ]]; then
    export SCENARIO="${SCENARIO}-gardenadm"
  fi

  echo "Detected scenario: $SCENARIO"
}

function skaffold_profile() {
  case "$1" in
    single-node)
      export SKAFFOLD_PROFILE="single-node"
      ;;
    single-node2)
      export SKAFFOLD_PROFILE="single-node2"
      ;;
    multi-node)
      export SKAFFOLD_PROFILE="multi-node"
      ;;
    multi-node-gardenadm)
      export SKAFFOLD_PROFILE="multi-node-gardenadm"
      ;;
    multi-node2)
      export SKAFFOLD_PROFILE="multi-node2"
      ;;
    multi-zone)
      export SKAFFOLD_PROFILE="multi-zone"
      ;;
    single-node-ipv6)
      export SKAFFOLD_PROFILE="single-node-ipv6"
      ;;
    multi-node-ipv6)
      export SKAFFOLD_PROFILE="multi-node-ipv6"
      ;;
    multi-zone-ipv6)
      export SKAFFOLD_PROFILE="multi-zone-ipv6"
      ;;
    single-node-dual)
      export SKAFFOLD_PROFILE="single-node-dual"
      ;;
    remote)
      export SKAFFOLD_PROFILE="remote"
      ;;
  esac

  if [[ -n "$SKAFFOLD_PROFILE" ]]; then
    echo "Using skaffold profile: $SKAFFOLD_PROFILE"
  fi
}

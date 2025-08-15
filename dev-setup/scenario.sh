#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

function detect_scenario() {
  nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n')
  zones=$(kubectl get nodes -o jsonpath='{.items[*].metadata.labels.topology\.kubernetes\.io/zone}' | tr ' ' '\n' | sort -u)

  if [[ $(echo "$nodes" | wc -l) -eq 1 ]]; then
    export SCENARIO="single-node"
  elif grep -q "gardener-local-multi-node2" <<< "$nodes"; then
    export SCENARIO="multi-node2"
  elif [[ $(echo "$zones" | wc -l) -eq 1 ]]; then
    export SCENARIO="multi-node"
  elif [[ $(echo "$zones" | wc -l) -eq 3 ]]; then
    export SCENARIO="multi-zone"
  else
    echo "Error: Unable to detect scenario. Please ensure you have a valid Kubernetes cluster with correctly labeled nodes with their availability zone." >&2
    exit 1
  fi

  if [[ "$IPFAMILY" == "ipv6" ]]; then
    export SCENARIO="${SCENARIO}-ipv6"
  fi

  echo "DETECTED SCENARIO: $SCENARIO"
}

function skaffold_profile() {
  case "$1" in
    single-node)
      export SKAFFOLD_PROFILE="single-node"
      ;;
    multi-node)
      export SKAFFOLD_PROFILE="multi-node"
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
  esac

  if [[ -n "$SKAFFOLD_PROFILE" ]]; then
    echo "USING SKAFFOLD PROFILE: $SKAFFOLD_PROFILE"
  fi
}

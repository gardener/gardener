#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> create-seed.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> <seed-name> ]"
  echo
  echo ">> For example: create-seed.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml seed-local"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2
seed_name=$3

registry_domain_file="$SCRIPT_DIR/registrydomain"
seed_values=values.yaml
if [[ "$seed_name" != "provider-extensions" ]]; then
  registry_domain_file="$SCRIPT_DIR/registrydomain-$seed_name"
  seed_values=values-"$seed_name".yaml
fi
registry_domain=$(cat "$registry_domain_file")

echo "Skaffolding seed"
GARDENER_LOCAL_KUBECONFIG=$garden_kubeconfig \
  SKAFFOLD_DEFAULT_REPO=$registry_domain \
  SEED_NAME=$seed_name \
  SEED_VALUES=$seed_values \
  SKAFFOLD_PUSH=true \
  skaffold run -m gardenlet -p extensions --kubeconfig="$seed_kubeconfig"

echo "Deploying additional kyverno policies"
kubectl --server-side=true --force-conflicts=true --kubeconfig="$seed_kubeconfig" apply -k "$SCRIPT_DIR/../registry-seed/kyverno-policies"

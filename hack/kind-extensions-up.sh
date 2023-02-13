#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -n "$(docker ps -aq -f name=gardener-extensions-control-plane)" ]]; then 
    docker start gardener-extensions-control-plane
else 
    ./hack/kind-up.sh --cluster-name gardener-extensions --environment "$KIND_ENV" --path-kubeconfig "${REPO_ROOT}/example/provider-extensions/garden/kubeconfig" --path-cluster-values "${REPO_ROOT}/example/gardener-local/kind/extensions/values.yaml"
fi

#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

if [[ -n "$(docker ps -aq -f name=gardener-extensions-control-plane)" ]]; then 
    docker start gardener-extensions-control-plane
else 
    ./hack/kind-up.sh --cluster-name gardener-extensions --path-kubeconfig "${REPO_ROOT}/example/provider-extensions/garden/kubeconfig" --path-cluster-values "${REPO_ROOT}/example/gardener-local/kind/extensions/values.yaml"
fi

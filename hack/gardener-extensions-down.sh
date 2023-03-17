#!/usr/bin/env bash
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

SEED_NAME=""
PATH_SEED_KUBECONFIG=""
PATH_GARDEN_KUBECONFIG=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --seed-name)
      shift; SEED_NAME="$1"
      ;;
    --path-seed-kubeconfig)
      shift; PATH_SEED_KUBECONFIG="$1"
      ;;
    --path-garden-kubeconfig)
      shift; PATH_GARDEN_KUBECONFIG="$1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

# Delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding.
# Deleting seed will fail as long as there are shoots scheduled on it. This is desired to ensure that there are no orphan infrastructure elements left.
echo "Deleting $SEED_NAME seed"
kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete seed "$SEED_NAME" --wait --ignore-not-found
skaffold --kubeconfig="$PATH_SEED_KUBECONFIG" delete -m gardenlet -p extensions
kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete ns "relay-$SEED_NAME" --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete ns garden registry relay --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete -k "$SCRIPT_DIR"/../example/provider-extensions/kyverno --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete mutatingwebhookconfigurations kyverno-policy-mutating-webhook-cfg kyverno-resource-mutating-webhook-cfg kyverno-verify-mutating-webhook-cfg --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete validatingwebhookconfigurations kyverno-policy-validating-webhook-cfg kyverno-resource-validating-webhook-cfg --ignore-not-found

remaining_seeds=$(kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" get seed --no-headers -o custom-columns=":metadata.name")
if [[ "$remaining_seeds" != "" ]]; then
  echo "No clean up of kind cluster because of remaining seeds: ${remaining_seeds//$'\n'/,}"
else
  echo "Cleaning up kind cluster"
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" annotate project local garden confirmation.gardener.cloud/deletion=true
  skaffold --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -m extensions-env -p extensions
  skaffold --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -m etcd,controlplane -p extensions
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete ns garden gardener-system-seed-lease --ignore-not-found
fi

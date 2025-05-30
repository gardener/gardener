#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

SEED_NAME=""
PATH_SEED_KUBECONFIG=""
PATH_GARDEN_KUBECONFIG=""
WORKLOAD_IDENTITY_SUPPORT=""

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
    --with-workload-identity-support)
      shift; WORKLOAD_IDENTITY_SUPPORT="$1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

client_certificate_data=$(kubectl config view --kubeconfig "$PATH_SEED_KUBECONFIG" --raw -o jsonpath='{.users[0].user.client-certificate-data}')
if [[ -n "$client_certificate_data" ]] && ! echo "$client_certificate_data" | base64 --decode | openssl x509 -noout -checkend 300 2>/dev/null ; then
  echo "Seed kubeconfig ${PATH_SEED_KUBECONFIG} has expired or will expire in 5min. Please provide a valid kubeconfig and try again!"
  exit 1
fi

# Delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding.
# Deleting seed will fail as long as there are shoots scheduled on it. This is desired to ensure that there are no orphan infrastructure elements left.
echo "Deleting $SEED_NAME seed"
kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete seed "$SEED_NAME" --wait --ignore-not-found
skaffold --kubeconfig="$PATH_SEED_KUBECONFIG" delete -m gardenlet -p extensions
kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete ns "relay-$SEED_NAME" --ignore-not-found
if [[ "$WORKLOAD_IDENTITY_SUPPORT" == "true" ]]; then
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -f "$SCRIPT_DIR"/../example/provider-extensions/gardener-discovery-server/rbac --ignore-not-found
fi
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete ns garden registry relay gardener-discovery-server --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete -k "$SCRIPT_DIR"/../example/provider-extensions/kyverno --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete mutatingwebhookconfigurations kyverno-policy-mutating-webhook-cfg kyverno-resource-mutating-webhook-cfg kyverno-verify-mutating-webhook-cfg --ignore-not-found
kubectl --kubeconfig "$PATH_SEED_KUBECONFIG" delete validatingwebhookconfigurations kyverno-policy-validating-webhook-cfg kyverno-resource-validating-webhook-cfg --ignore-not-found

remaining_seeds=$(kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" get seed --no-headers -o custom-columns=":metadata.name")
if [[ "$remaining_seeds" != "" ]]; then
  echo "No clean up of kind cluster because of remaining seeds: ${remaining_seeds//$'\n'/,}"
else
  echo "Cleaning up admission controllers"
  "$SCRIPT_DIR"/../example/provider-extensions/garden/configure-admission.sh "$PATH_GARDEN_KUBECONFIG" delete --ignore-not-found
  echo "Cleaning up kind cluster"
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" annotate project garden confirmation.gardener.cloud/deletion=true
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" annotate -f "$SCRIPT_DIR"/../example/provider-extensions/garden/project/project.yaml confirmation.gardener.cloud/deletion=true
  skaffold --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -m extensions-env -p extensions
  skaffold --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -m etcd,controlplane -p extensions
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete ns garden gardener-system-seed-lease gardener-system-shoot-issuer gardener-system-public --ignore-not-found
  kubectl --kubeconfig="$PATH_GARDEN_KUBECONFIG" delete -f "$SCRIPT_DIR"/../example/provider-extensions/gardener-discovery-server/rbac --ignore-not-found
fi

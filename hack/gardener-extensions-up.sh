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

echo "Configure seed cluster"
"$SCRIPT_DIR"/../example/provider-extensions/seed/configure-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG" "$SEED_NAME" "$WORKLOAD_IDENTITY_SUPPORT"
echo "Start bootstrapping Gardener"
SKAFFOLD_DEFAULT_REPO=garden.local.gardener.cloud:5001 SKAFFOLD_PUSH=true skaffold run -m etcd,controlplane,extensions-env -p extensions
echo "Configure admission controllers"
"$SCRIPT_DIR"/../example/provider-extensions/garden/configure-admission.sh "$PATH_GARDEN_KUBECONFIG" apply
echo "Registering controllers"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true --force-conflicts=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/controllerregistrations
echo "Creating CloudProfiles"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/cloudprofiles

if [[ "$WORKLOAD_IDENTITY_SUPPORT" == "true" ]]; then
  "$SCRIPT_DIR"/../example/provider-extensions/seed/configure-discovery-server.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG"
fi

"$SCRIPT_DIR"/../example/provider-extensions/seed/create-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG" "$SEED_NAME"

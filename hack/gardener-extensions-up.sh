#!/usr/bin/env bash

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

echo "Configure seed cluster"
"$SCRIPT_DIR"/../example/provider-extensions/seed/configure-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG" "$SEED_NAME"
echo "Start bootstrapping Gardener"
SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true skaffold run -m etcd,controlplane,extensions-env -p extensions
echo "Registering controllers"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/controllerregistrations
echo "Creating CloudProfiles"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/cloudprofiles
"$SCRIPT_DIR"/../example/provider-extensions/seed/create-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG"

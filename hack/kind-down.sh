#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME=""
PATH_KUBECONFIG=""
KEEP_BACKUPBUCKETS_DIRECTORY=false

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --cluster-name)
      shift; CLUSTER_NAME="$1"
      ;;
    --path-kubeconfig)
      shift; PATH_KUBECONFIG="$1"
      ;;
    --keep-backupbuckets-dir)
      KEEP_BACKUPBUCKETS_DIRECTORY=false
      ;;
    esac

    shift
  done
}

parse_flags "$@"

kind delete cluster \
  --name "$CLUSTER_NAME"

if [[ "$CLUSTER_NAME" != "gardener-local2" ]]; then
  # Only stop the infra containers if deleting the "main" kind cluster.
  # When deleting the secondary cluster, we might still need DNS/registry for the other cluster.
  # Reset dynamic updates to the DNS zones by removing the volumes.
  docker compose -f "$(dirname "$0")/../dev-setup/infra/docker-compose.yaml" down --volumes

  # When deleting the "main" kind cluster, remove all load balancer containers (including the ones of shoot clusters)
  # to remove any orphaned containers.
  echo "Removing load balancer containers of all clusters"
  for container in $(docker container ls -aq --filter network=kind --filter label=gardener.cloud/role=loadbalancer); do
    docker container rm -f "$container"
  done
else
  echo "Removing load balancer containers of cluster $CLUSTER_NAME"
  for container in $(docker container ls -aq --filter network=kind --filter label=gardener.cloud/role=loadbalancer --filter label=kubernetes.io/cluster="$CLUSTER_NAME"); do
    docker container rm -f "$container"
  done
fi

rm -f "$PATH_KUBECONFIG"
if [[ "$PATH_KUBECONFIG" == *"dev-setup/gardenlet/components/kubeconfigs/seed-local2/kubeconfig" ]]; then
  rm -f "${PATH_KUBECONFIG}-gardener-operator"
fi

if [[ "$KEEP_BACKUPBUCKETS_DIRECTORY" == "false" ]]; then
  rm -rf "$(dirname "$0")/../dev/local-backupbuckets"
fi

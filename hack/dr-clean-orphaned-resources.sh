#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# Removes the orphaned resources that a previous disaster-recovery run left behind in the reusable kind
# cluster (Shoot, ShootState, BackupEntry, BackupBucket) as well as the on-disk local backup bucket data.
#
# The gardenlet that owns the finalizers on these resources runs on the self-hosted shoot machine, which
# is destroyed during the disaster simulation and never rejoins the garden cluster. Hence no controller is
# left to process a graceful deletion, and we have to force-remove the finalizers here.
#
# This script is idempotent and can be run repeatedly. Run it before starting another DR run when a
# previous run left resources behind.

VIRTUAL_GARDEN_KUBECONFIG="${VIRTUAL_GARDEN_KUBECONFIG:-./dev-setup/kubeconfigs/virtual-garden/kubeconfig}"
SHOOT_NAME="${SHOOT_NAME:-root}"
SHOOT_NAMESPACE="${SHOOT_NAMESPACE:-garden}"
DIR_BACKUP_BUCKET="${DIR_BACKUP_BUCKET:-./dev/local-backupbuckets}"

kubectl_virtual() {
  kubectl --kubeconfig "$VIRTUAL_GARDEN_KUBECONFIG" "$@"
}

# force_delete <kind> [-n <namespace>] <name>
# Annotates the object for deletion confirmation, requests deletion without waiting and then strips all
# finalizers so the object is removed even though no controller is around to process them.
force_delete() {
  local kind="$1"; shift
  local ns_args=()
  if [[ "${1:-}" == "-n" ]]; then
    ns_args=("$1" "$2"); shift 2
  fi
  local name="$1"

  if ! kubectl_virtual "${ns_args[@]}" get "$kind" "$name" &>/dev/null; then
    return 0
  fi

  echo "> Removing $kind/$name..."
  # A Shoot's `gardener` finalizer can only be removed once its status.lastOperation is a successful
  # Delete (enforced by the FinalizerRemoval admission plugin). An orphaned Shoot still carries its
  # last reconcile (e.g. Create/Succeeded), so make the status look like a completed deletion first.
  if [[ "$kind" == "shoot" ]]; then
    kubectl_virtual "${ns_args[@]}" patch "$kind" "$name" --subresource=status --type=merge \
      -p '{"status":{"lastOperation":{"type":"Delete","state":"Succeeded","progress":100,"description":"Force-cleaned by dr-clean-orphaned-resources.sh"}}}'
  fi
  kubectl_virtual "${ns_args[@]}" annotate "$kind" "$name" confirmation.gardener.cloud/deletion=true --overwrite
  kubectl_virtual "${ns_args[@]}" patch "$kind" "$name" --type=merge -p '{"metadata":{"finalizers":null}}'
  kubectl_virtual "${ns_args[@]}" delete "$kind" "$name" --ignore-not-found
}

echo "> Cleaning up orphaned resources from previous disaster-recovery runs..."

if [[ ! -f "$VIRTUAL_GARDEN_KUBECONFIG" ]] || ! kubectl_virtual get namespaces &>/dev/null; then
  echo "> Virtual garden cluster is not reachable via $VIRTUAL_GARDEN_KUBECONFIG; skipping garden resource cleanup."
else
  # Only the resources that `gardenadm connect` and the gardenlet create per DR run are removed here:
  # the Shoot, its ShootState, the ControllerInstallations and the Backup{Entry,Bucket}.
  #
  # The shared control plane configuration (CloudProfile `local`, ControllerRegistrations,
  # ControllerDeployments, the `garden` Project and Namespace) is deliberately NOT deleted. It is created
  # once by `make gardenadm-up SCENARIO=connect-kind` (via the connect manifests and gardener-operator) and
  # is reused across DR runs - deleting it would force a full control plane re-provisioning before the next
  # run, defeating the purpose of reusing the kind cluster.
  force_delete shoot -n "$SHOOT_NAMESPACE" "$SHOOT_NAME"
  force_delete shootstate -n "$SHOOT_NAMESPACE" "$SHOOT_NAME"

  for ci in $(kubectl_virtual get controllerinstallations -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
    force_delete controllerinstallation "$ci"
  done

  for be in $(kubectl_virtual -n "$SHOOT_NAMESPACE" get backupentries -o name 2>/dev/null); do
    force_delete backupentry -n "$SHOOT_NAMESPACE" "${be#*/}"
  done

  # BackupBuckets are cluster-scoped. Keep the garden-* bucket, which belongs to the virtual garden control
  # plane running on the kind cluster and must survive across DR runs.
  for bb in $(kubectl_virtual get backupbuckets -o name 2>/dev/null); do
    name="${bb#*/}"
    if [[ "$name" == garden-* ]]; then
      continue
    fi
    force_delete backupbucket "$name"
  done
fi

# Remove the on-disk backup bucket data of the self-hosted shoot(s), keeping the garden-* bucket of the
# virtual garden control plane. The backup files are written by containers running as root, so we delete
# them from within a root container (same approach as dev-setup/infra.sh).
if [[ -d "$DIR_BACKUP_BUCKET" ]]; then
  echo "> Removing on-disk backup bucket data (keeping the garden-* control plane bucket)..."
  docker run --rm --user root:root -v "$DIR_BACKUP_BUCKET":/local-backupbuckets alpine \
    sh -c 'find /local-backupbuckets -mindepth 1 -maxdepth 1 -type d ! -name "garden-*" -exec rm -rf {} +'
fi

echo "> Cleanup complete."

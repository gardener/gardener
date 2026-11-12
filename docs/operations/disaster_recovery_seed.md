---
title: "Disaster Recovery: Restoring Shoot Control Planes on the Same Seed"
description: How to restore Shoot control planes in place after a Seed etcd disaster using the force-restore operation
categories:
  - Operators
---

# Disaster Recovery: Restoring Shoot Control Planes on the Same Seed

This document describes how to recover Shoot control planes after an etcd disaster on a Seed cluster, when the Seed itself is recovered in place (i.e., reusing the same backup bucket rather than migrating to a different Seed).

This applies to unmanaged Seeds (i.e., Seeds that are not backed by a `ManagedSeed` resource).

## Background

The standard Gardener [control plane migration](./control_plane_migration.md) flow copies ETCD backups from the source Seed's backup bucket to the destination Seed's backup bucket. When recovering on the **same** Seed, this copy step is unnecessary. The `force-restore` operation annotation skips the backup copy and restores the control plane from the existing backups.

## Prerequisites

- The Seed, Shoot, BackupBucket, and BackupEntry resources still exist in the garden cluster. Preserve these resources during recovery; recreating a Seed with the same name does not preserve its UID.
- Each affected Shoot's `.spec.seedName` and `.status.seedName` refer to the recovered Seed. Do not change the Shoot's Seed assignment during this procedure.
- The Seed cluster can be recovered in place (same backup bucket), and its gardenlet can be brought back up.
- Each Shoot's etcd data was backed up by `etcd-backup-restore` to the backup bucket before the disaster, and the backup bucket is still intact and reachable.
- A current `ShootState` exists in the garden cluster for each affected Shoot. Enable the gardenlet's periodic ShootState backups (`controllers.shootState.concurrentSyncs > 0`) before the disaster. The restore uses the saved credentials and extension state, including infrastructure and machines.

> [!IMPORTANT]
> Set the emergency stop (Step 1) **before** the recovered gardenlet comes back up. A Shoot's
> `status.lastOperation` lives in the garden cluster and survives the Seed etcd loss as `Reconcile`
> `Succeeded`. If gardenlet starts before the emergency stop is in place, it computes an ordinary
> `Reconcile` (the `force-restore` annotation is not set yet) and may regenerate the control plane's
> secrets against the empty Seed. Avoid ordinary reconciliation until restoration is requested.
> Applying the emergency stop does not require the Seed cluster to be up.

## Recovery Procedure

### 1. Stop Shoot Reconciliations on the Seed (before recovering the Seed)

Before recovering the Seed cluster, prevent gardenlet from reconciling Shoots once it comes back. Annotate the Seed resource in the garden cluster:

```bash
kubectl annotate seed <seed-name> shoot.gardener.cloud/emergency-stop-reconciliations=true --overwrite
```

This blocks gardenlet from picking up any Shoot reconciliation until you lift the annotation, so it cannot start an ordinary reconcile against the freshly recovered (empty) etcd.

### 2. Recover the Seed Cluster

Recover the Seed cluster in place (reusing the same backup bucket) and bring its gardenlet back up. Because the emergency stop from Step 1 is already in place, gardenlet will not reconcile any Shoots yet. Wait until the Seed reports ready (`GardenletReady` and `SeedSystemComponentsHealthy` conditions are `True`).

### 3. Annotate Each Affected Shoot with `force-restore`

For each Shoot scheduled on the Seed, set the `force-restore` operation annotation:

```bash
kubectl -n <project-namespace> annotate shoot <shoot-name> gardener.cloud/operation=force-restore --overwrite
```

The Gardener API server will automatically bump the Shoot's generation when this annotation is set, so gardenlet will reconcile it once the emergency stop is removed.

List the affected Shoots, then annotate each one using the command above:

```bash
kubectl get shoots --all-namespaces -o json | \
  jq -r --arg seed '<seed-name>' '.items[] | select(.spec.seedName == $seed) | [.metadata.namespace, .metadata.name] | @tsv'
```

Do not apply `force-restore` to Shoots on healthy Seeds or combine it with other operation annotations.

### 4. Lift the Emergency Stop

Once all affected Shoots are annotated, remove the emergency stop from the Seed to allow reconciliation to proceed:

```bash
kubectl annotate seed <seed-name> shoot.gardener.cloud/emergency-stop-reconciliations-
```

Gardenlet will now start reconciling the annotated Shoots. For each one, it:

1. Detects the `force-restore` annotation and maps it to a Restore operation.
2. Skips the backup copy step (the backup bucket is unchanged).
3. Restores credentials and extension state from the ShootState, including the BackupEntry extension that recreates the `etcd-backup` Secret.
4. Deploys ETCD, recovering its data from the existing snapshots and delta snapshots.
5. Removes the `force-restore` annotation once ETCD is ready.
6. Continues restoring the remaining control plane and system components.

### 5. Monitor Progress

Watch the Shoot status for each cluster:

```bash
kubectl -n <project-namespace> get shoot <shoot-name> -w
```

A Shoot in restore phase will have `status.lastOperation.type: Restore`.

## Notes

- **Generation bump on annotation**: the Gardener API server bumps the Shoot generation when `force-restore` is set, including when the previous operation is `Failed`. This triggers reconciliation without a spec change. The Shoot must already have a `status.lastOperation`.
- **Annotation removal**: gardenlet removes the `force-restore` annotation automatically once etcd is ready. No manual cleanup is needed.
- **Self-hosted Shoots**: the Gardener API server rejects `force-restore` for self-hosted Shoots. These follow a different recovery path.

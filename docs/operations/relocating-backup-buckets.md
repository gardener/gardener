# Relocating a Backup Bucket

An operator may need to relocate the etcd backup target of a `Seed`, `Garden`, or self-hosted `Shoot` to a new cloud provider or a different region.
However, the provider and region of an existing `BackupBucket` resource are immutable. Further, changes to the backup configuration of a `Seed`, `Garden`, or self-hosted `Shoot` are protected against accidental updates.

To change the backup target, an operator needs to trigger the creation of a new `BackupBucket` by changing the `backup` configuration.
For this, it is **mandatory to change the `bucketName` field**. Optionally, the `provider` and `region` fields can be changed on top of that.

> [!NOTE]
> Some providers require the `bucketName` to be globally unique, e.g. a UUID.

Refer to the following table to find the resource to update.

| Scenario | Resource to update | Backup configuration path | Confirmation annotation path |
| --- | --- | --- | --- |
| Seed managed through a `ManagedSeed` (i.e. seed is a shoot) | `ManagedSeed` | `.spec.gardenlet.config.seedConfig.spec.backup` | `.spec.gardenlet.config.seedConfig.metadata.annotations` |
| Seed managed through a `Gardenlet` (i.e. on the soil cluster) | `Gardenlet` | `.spec.config.seedConfig.spec.backup` | `.spec.config.seedConfig.metadata.annotations` |
| Virtual garden | `Garden` | `.spec.virtualCluster.etcd.main.backup` | `.metadata.annotations` |
| Self-hosted shoot | `Shoot` | `.spec.provider.workers[].controlPlane.backup` (select the worker pool with the `controlPlane` section) | `.metadata.annotations` |

> [!NOTE]
> Do not edit the configuration directly in the `Seed`, because these changes will conflict with the configuration in the managing resource.

The changes must be confirmed by setting the following annotation:
```yaml
confirmation.gardener.cloud/change-backup: "true"
```
The annotation must be set at the location shown in the table, i.e. for a `Garden` or self-hosted `Shoot`, set the annotation in the resource's top-level `.metadata.annotations`.
For a `ManagedSeed` or `Gardenlet`, set the annotation in the embedded `seedConfig.metadata.annotations`, not in the resource's top-level `.metadata.annotations`.

## Propagation of the Change
For a `Seed`, the updated `ManagedSeed` or `Gardenlet` configuration is propagated to the gardenlet, which updates the `Seed` and reconciles the new `BackupBucket`.
For a virtual garden, gardener-operator reconciles the new backup target from the updated `Garden` resource.
The change will take effect in etcd on the next `Shoot` or `Garden` reconciliation, respectively. A `Shoot` reconciliation is not automatically triggered by changing the backup configuration in the `ManagedSeed` or `Gardenlet`.
For a self-hosted `Shoot`, changing the backup configuration in the Shoot spec triggers Shoot reconciliation.

## Cleaning up Leftover Buckets
Deleting the old bucket is an operator-driven decision and is done only when the operator is confident it is no longer needed.

## Risks and Recovery

> [!WARNING]
> Relocation rolls the etcd Pod. If its data becomes corrupted in the process and the new backup location contains no snapshots, recovery may bootstrap an empty database, leaving the Shoot or Garden unusable.
>
> Before reverting the backup configuration, it may be advisable to manually save a separate copy of the known-good snapshots from the old location to a safe place.
>
> After that, restore the complete old backup configuration, including the bucket, provider, region, credentials, and prefix. For single-member etcd, startup validation can automatically restore the old backup if the local database revision is behind the backup revision. If automatic recovery does not restore the expected data, follow the [manual etcd restoration procedure](https://github.com/gardener/etcd-backup-restore/blob/master/docs/operations/manual_restoration.md).
>
> Retry relocation only after the expected data is restored, the cluster is healthy, and a fresh backup of the recovered database has succeeded.
>
> This corruption scenario is rare, but the risk is highest for standalone (non-HA) etcd. Running etcd in an HA setup makes the operation considerably safer, since a single member's volume corruption does not brick the cluster.

## Note on the Mutability of BackupBucket Spec Fields
This spec-field mutability is unrelated to [Immutable Backup Buckets](../../operations/immutable-backup-buckets.md), which is an opt-in WORM/retention feature protecting the stored snapshot *objects*. If the old bucket is under a **locked** retention policy, it (and its objects) cannot be deleted until the retention period expires, so cleanup after relocation may not be immediate.

---
title: Immutable Backup Buckets
---

# Immutable Backup Buckets

## Overview 

Immutable backup buckets ensure that etcd backups cannot be modified or deleted after creation, leveraging immutability features provided by supported cloud providers. This is essential for meeting security, compliance, and operational requirements.

> [!NOTE]
> For background, see [Gardener Issue #10866](https://github.com/gardener/gardener/issues/10866).

## Core Concepts

Gardener supports immutable backup buckets for `AWS`, `GCP`, and `Azure` via their respective provider extensions. When enabled, the extension will:

- **Create** buckets with the specified immutability policy if they do not exist.
- **Reconcile** existing buckets to enforce the desired immutability policy.
- **Prevent** reducing the retention period or disabling immutability once the policy is locked.
- **Apply** lifecycle-based delayed deletion on `GCP` and `Azure` when immediate deletion is not permitted.

> [!IMPORTANT]
> Once immutability is locked, it cannot be disabled or shortened.

## Admission Webhook Enforcement

An admission webhook ensures:

- **Locking:** Once `locked: true`, immutability cannot be disabled.
- **Retention:** Once the retention period is set and locked, it cannot be decreased.

## Configuration

To enable immutable backup buckets, specify the immutability settings in `.spec.backup.providerConfig` in the `Seed` resource. The gardenlet will automatically create or update the bucket according to the configuration.

- See [BackupBucket Resource Contract](../../extensions/resources/backupbucket.md)
- See [Gardenlet Controller Concepts – BackupBucket Controller](../../concepts/gardenlet.md#backupbucket-controller)

## Example Configurations

### `GCP`

```yaml
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: BackupBucketConfig
metadata:
  name: example-immutable-backup
immutability:
  retentionPeriod: 96h        # Backups are immutable for 4 days
  retentionType: bucket       # Bucket-level immutability
  locked: false               # Set to true to lock the policy
```

> [!NOTE]
> `GCP` uses lifecycle rules to delay object deletion until the retention period expires. Once `locked: true` is set, the policy cannot be reverted or shortened. See [`GCP` Extension Usage – BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#backupbucketconfig)

### `AWS`

```yaml
apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
kind: BackupBucketConfig
metadata:
  name: example-immutable-backup
immutability:
  retentionType: bucket  # Enable S3 Object Lock on bucket
  retentionPeriod: 96h  # Backups are immutable for 4 days
  mode: compliance     # or governance
```

> [!NOTE]
> `AWS` S3 Object Lock in **COMPLIANCE** mode cannot be overridden by any user, including the bucket owner. See [`AWS` Extension Usage – BackupBucket](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#backupbucket)

### Azure

```yaml
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: BackupBucketConfig
metadata:
  name: example-immutable-backup
immutability:
  retentionPeriod: 96h        # Backups are immutable for 4 days
  retentionType: bucket       # Container-level immutability
  locked: true                # Once enabled, policy is irreversible
```

> [!NOTE]
> `Azure` uses container immutability policies with delayed deletion. Once `locked: true` is set, the policy cannot be changed or removed.  
> See [`Azure` Extension Usage – BackupBucket](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#backupbucket)

## Advanced: Ignoring Snapshots During Restoration

Immutable buckets prevent snapshot deletion. If you encounter issues during restore, you may need to skip problematic snapshots.

See [etcd-backup-restore: Ignoring Snapshots During Restoration](https://github.com/gardener/etcd-backup-restore/blob/master/docs/usage/enabling_immutable_snapshots.md#ignoring-snapshots-during-restoration)

## References

- [BackupBucket Resource Contract](../../extensions/resources/backupbucket.md)
- [Gardenlet Controller Concepts](../../concepts/gardenlet.md#backupbucket-controller)
- [`GCP` Extension Usage – BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#backupbucketconfig)
- [`AWS` Extension Usage – BackupBucket](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#backupbucket)
- [`Azure` Extension Usage – BackupBucket](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#backupbucket)
- [etcd-backup-restore: Enabling Immutable Snapshots](https://github.com/gardener/etcd-backup-restore/blob/master/docs/usage/enabling_immutable_snapshots.md)

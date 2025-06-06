---
title: Immutable Backup Buckets
---

# Immutable Backup Buckets

## Overview

**Immutable backup buckets** ensure that etcd backups cannot be modified or deleted before the configured retention period expires, by leveraging immutability features provided by supported cloud providers.

This capability is critical for:

* **Security**: Protecting against accidental or malicious deletion of backups.
* **Compliance**: Meeting regulatory requirements for data retention.
* **Operational integrity**: Ensuring recoverable state of your Kubernetes clusters.

> [!NOTE]
> For background, see [Gardener Issue #10866](https://github.com/gardener/gardener/issues/10866).

## Core Concepts

When **immutability** is enabled via a supported Gardener provider extension:

The **provider extension** will:

* ✅ **Create** the backup bucket with the desired immutability policy (if it does not already exist).
* 🔄 **Reconcile** the policy on existing buckets to match the current configuration.
* 🚫 **Prevent changes** that would weaken the policy (reduce retention period or disable immutability).
* 🕑 **Manage deletion lifecycle**: If retention lock prevents immediate deletion of objects, a deletion policy will apply when allowed.

> \[!IMPORTANT]
> Once a bucket's immutability is **locked** at the cloud provider level, it cannot be removed or shortened—even by administrators or operators.

## Provider Support
> [!WARNING]
> **Not all Gardener provider extensions currently support immutable buckets.**
Support and configuration options vary between providers.

Please check your cloud provider’s extension documentation (see [References](#references)) for up-to-date support and syntax.

## How It Works: Admission Webhook Enforcement

To ensure backup integrity, an **admission webhook** enforces:

* 🔒 **Immutability lock**: Once the policy is locked, it cannot be disabled.
* 📅 **Retention period**: Once the policy is locked, the retention period cannot be shortened.

This protects clusters from accidental misconfiguration or policy drift.

## How to Enable Immutable Backup Buckets

To enable immutable backup buckets:
1️⃣ Set the **immutability options** in `.spec.backup.providerConfig` of your `Seed` resource.
2️⃣ The **Gardenlet** and the provider extension will collaborate to provision and manage the bucket.

### Example Configuration

Below is a **generic example**; adjust according to your cloud provider’s API:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
metadata:
  name: example-seed
spec:
  backup:
    provider: <provider-name>
    providerConfig:
      apiVersion: <provider-extension-api-version>
      kind: BackupBucketConfig
      immutability:
        retentionPeriod: 96h          # Required: Retention duration (e.g., 96h)
        retentionType: bucket         # Bucket-level or object-level (provider-specific)
        locked: false                 # Whether to lock policy on creation (recommended: true for production)
```

> [!NOTE]
> See your provider’s documentation for specific fields and behavior:
>
> * [GCP BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#backupbucketconfig)
> * [AWS BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#backupbucketconfig)
> * [Azure BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#backupbucketconfig)


## Advanced: Ignoring Snapshots During Restoration

When using immutable backup buckets, you may encounter situations where certain snapshots cannot be deleted due to immutability constraints. In such cases, you can configure the etcd-backup-restore tool to **ignore problematic snapshots** during restoration.
This allows you to proceed with restoring the etcd cluster without being blocked by snapshots that cannot be deleted.

> [!WARNING]
> **Ignoring snapshots should be used with caution**. It is recommended to only ignore snapshots that you are certain are not needed for recovery, as this may lead to data loss if critical snapshots are skipped.

👉 See: [Ignoring Snapshots During Restoration](https://github.com/gardener/etcd-backup-restore/blob/master/docs/usage/enabling_immutable_snapshots.md#ignoring-snapshots-during-restoration).


## References

* [BackupBucket Resource Contract](../../extensions/resources/backupbucket.md)
* [Gardenlet Controller Concepts – BackupBucket Controller](../../concepts/gardenlet.md#backupbucket-controller)
* [GCP BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/docs/usage/usage.md#backupbucketconfig)
* [AWS BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage/usage.md#backupbucketconfig)
* [Azure BackupBucketConfig](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#backupbucketconfig)
* [etcd-backup-restore: Enabling Immutable Snapshots](https://github.com/gardener/etcd-backup-restore/blob/master/docs/usage/enabling_immutable_snapshots.md)
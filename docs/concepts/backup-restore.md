---
title: Backup and Restore
description: Understand the etcd backup and restore capabilities of Gardener
categories:
  - Users
---

## Overview

Kubernetes uses [etcd](https://etcd.io/) as the key-value store for its resource definitions. Gardener supports the backup and restore of etcd. It is the responsibility of the shoot owners to backup the workload data.

Gardener uses an [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) component to backup the etcd backing the Shoot cluster regularly and restore it in case of disaster. It is deployed as sidecar via [etcd-druid](https://github.com/gardener/etcd-druid). This doc mainly focuses on the backup and restore configuration used by Gardener when deploying these components. For more details on the design and internal implementation details, please refer to [GEP-06](../proposals/06-etcd-druid.md) and the documentation on individual repositories.

## Bucket Provisioning
Refer to the [backup bucket extension document](../extensions/resources/backupbucket.md) to find out details about configuring the backup bucket.

## Backup Policy
etcd-backup-restore supports full snapshot and delta snapshots over full snapshot. In Gardener, this configuration is currently hard-coded to the following parameters:

- Full Snapshot schedule:
    - Daily, `24hr` interval.
    - For each Shoot, the schedule time in a day is randomized based on the configured Shoot maintenance window.
- Delta Snapshot schedule:
    - At `5min` interval.
    - If aggregated events size since last snapshot goes beyond `100Mib`.
- Backup History / Garbage backup deletion policy:
    - Gardener configures backup restore to have `Exponential` garbage collection policy.
    - As per policy, the following backups are retained:
      - All full backups and delta backups for the previous hour.
      - Latest full snapshot of each previous hour for the day.
      - Latest full snapshot of each previous day for 7 days.
      - Latest full snapshot of the previous 4 weeks.
    - Garbage Collection is configured at `12hr` interval.
- Listing:
    - Gardener doesn't have any API to list out the backups.
    - To find the backups list, an admin can checkout the `BackupEntry` resource associated with the Shoot which holds the bucket and prefix details on the object store.
 
## Manual Backup of etcd-encryption-keys
Be sure to manually backup the contents of your kube-apiserver-etcd-encryption-key and gardener-apiserver-etcd-encryption-key secrets if your disaster recovery scenario includes restoration of the virtual-garden's etcd.

## Restoration of shoots
The restoration process of etcd is automated through the etcd-backup-restore component from the latest snapshot. Gardener doesn't support Point-In-Time-Recovery (PITR) of etcd. In case of an etcd disaster, the etcd is recovered from the latest backup automatically. For further details, please refer the [Restoration](https://github.com/gardener/etcd-backup-restore/blob/master/docs/proposals/restoration.md) topic. Post restoration of etcd, the Shoot reconciliation loop brings the cluster back to its previous state.

Again, the Shoot owner is responsible for maintaining the backup/restore of his workload. Gardener only takes care of the cluster's etcd.

## Restoration of virtual-garden in a new cluster
During installation of your disaster recovery cluster, hint the secrets-manager to reuse your old clusters etcd encryption keys for kube-apiserver and gardener-apiserver. The following can be understood as an example to help with this undertaking: 
```
---
apiVersion: v1
data: {"encoding":"bm9uZQ==","key":"<your_key_1>","secret":"<your_secret_1>"}
kind: Secret
metadata:
  labels:
    secrets-manager-use-data-for-name: kube-apiserver-etcd-encryption-key
  name: kube-apiserver-etcd-encryption-key-initial
  namespace: garden
type: Opaque
---
apiVersion: v1
data: {"encoding":"bm9uZQ==","key":"<your_key_2>","secret":"<your_secret_2>"}
kind: Secret
metadata:
  labels:
    secrets-manager-use-data-for-name: gardener-apiserver-etcd-encryption-key
  name: gardener-apiserver-etcd-encryption-key-initial
  namespace: garden
type: Opaque
```
Be sure to adjust your gardens.operator.gardener.cloud beforehand to disable the `topologyAwareRouting` and `controlPlane.highAvailability` in case those settings were featured in your to-be-restored cluster. Feel free to reenable them once the restoration succeeded.

<next steps to be determined, stay tuned while we figure this our together <3 >

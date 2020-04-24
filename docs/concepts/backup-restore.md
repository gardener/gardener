# Backup and restore

Kubernetes uses Etcd as the key-value store for its resource definitions. Gardener supports the backup and restore of etcd. It is the responsibility of the shoot owners to backup the workload data.

Gardener uses [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) component to backup the etcd backing the Shoot cluster regularly and restore in case of disaster. It is deployed as sidecar via [etcd-druid](https://github.com/gardener/etcd-druid). This doc mainly focuses on the backup and restore configuration used by Gardener when deploying these components. For more details on the design and internal implementation details, please refer [GEP-06](../proposals/06-etcd-druid.md) and documentation on individual repository.

## Bucket provisioning
Refer the [backup bucket extension document](../extensions/backupbucket.md) to know details about configuring backup bucket.

## Backup Policy
etcd-backup-restore supports full snapshot and delta snapshots over full snapshot. In Gardener, this configuration is currently hard-coded to following parameters:

- Full Snapshot Schedule:
    - Daily, `24hr` interval.
    - For each Shoot, the schedule time in a day is randomized based on the configured Shoot maintenance window.
- Delta Snapshot schedule:
    - At `5min` interval.
    - If aggregated events size since last snapshot goes beyond `100Mib`.
- Backup History / Garbage backup deletion policy:
    - Gardener configure backup restore to have `Exponential` garbage collection policy.
    - As per policy, following backups are retained.
    - All full backups and delta backups for the previous hour.
    - Latest full snapshot of each previous hour for the day.
    - Latest full snapshot of each previous day for 7 days.
    - Latest full snapshot of the previous 4 weeks.
    - Garbage Collection is configured at `12hr` interval.
- Listing:
    - Gardener don't have any API to list out the backups.
    - To find the backup list, admin can checkout the `BackupEntry` resource associated with Shoot which holds the bucket and prefix details on object store.

## Restoration
Restoration process of etcd is automated through the etcd-backup-restore component from latest snapshot. Gardener dosen't support Point-In-Time-Recovery (PITR) of etcd. In case of etcd disaster, the etcd is recovered from latest backup automatically. For further details, please refer the [doc](https://github.com/gardener/etcd-backup-restore/blob/master/doc/proposals/restoration.md). Post restoration of etcd, the Shoot reconciliation loop brings back the cluster to same state.

Again, Shoot owner is responsible for maintaining the backup/restore of his workload. Gardener does only take care of the cluster's etcd.

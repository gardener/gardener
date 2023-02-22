# Troubleshooting Guide

## Are there really issues that cannot be fixed?

Well, of course not. With the continuous development of Gardener, over the time its architecture and API might have to be changed to reduce complexity and support more features. In this process, developers are bound to keep the current Gardener version backward compatible with the last two releases. But maintaining backward compatibility is quite the complex and effortful task. So, to save the short term complex effort, it's common practice in open source community to use workarounds or hacky solutions sometimes. This results in rare issues which are supposed to be resolved by human interaction across upgrades of the Gardener version.

This guide records the issues that are quite possible across upgrade of Gardener version, their root cause and the human action required for graceful resolution of the issue. For a troubleshooting guide of bugs which are not yet fixed, please refer the associated GitHub issue.

> **Note To Maintainers:** Please only mention the resolution of issues which are by design. For bugs, please report the temporary resolution in the GitHub issue created for the bug.

### Etcd-Main pod fails to come up since the backup-restore sidecar is reporting RevisionConsistencyCheckErr

#### Issue

- Etcd-main pod goes in `CrashLoopBackoff`.
- Etcd-backup-restore sidecar reports validation error with RevisionConsistencyCheckErr.

#### Environment

- Gardener version: 0.29.0+

#### Root Cause

- From version 0.29.0, Gardener uses a shared backup bucket for storing etcd backups, replacing the old logic of having a single bucket per shoot as per [GEP-02](../proposals/02-backupinfra.md).
- Since there are very rare chances that the etcd data directory will get corrupted while doing this migration, to avoid etcd downtime and implementation effort, we decided to switch directly from the old bucket to the new shared bucket without migrating the snapshot from the old bucket to the new bucket.
- In this case, just to be on the safe side, we added a sanity check in the etcd-backup-restore sidecar of the etcd-main pod, which checks if the etcd data revision is greater than the last snapshot revision from the old bucket.
- If the above check fails, this means that some data corruption has occurred with etcd, so etcd-backup-restore reports error and then etcd-main pod goes in `CrashLoopBackoff`, creating etcd-main down alerts.

#### Action

1. Disable the Gardener reconciliation for the Shoot by annotating it with `shoot.gardener.cloud/ignore=true`.
2. Scale down the etcd-main statefulset in the seed cluster.
3. Find out the latest full snapshot and delta snapshot from the old backup bucket. The old backup bucket name is the same as the backupInfra resource associated with the Shoot in the Garden cluster.
4. Move them manually to new the backup bucket.
5. Enable the Gardener reconciliation for the shoot by removing the `shoot.gardener.cloud/ignore=true` annotation.

# Trouble Shooting Guide

## Are there really issue that cannot be fixed :O?
Well, of course not :P. With continuous development of Gardener, over the time its architecture and API might have to be changed to reduce complexity and support more features. In this process developers are bound to keep Gardener version backward compatible with last two releases. But maintaining backward compatibility is quite complex and effortful tasks. So, to save short term complex effort, its common practice in open source community to use work around or hacky solutions sometimes. This results in rare issues which are supposed to be resolved by human interaction across upgrades of Gardener version.

This guide records the issues that are quite possible across upgrade of Gardener version, root cause and the human action required for graceful resolution of issue. For troubleshooting guide of bugs which are not yet fixed, please refer the associated github issue.

**Note To Maintainers:** Please use only mention the resolution of issues which are by design. For bugs please report the temporary resolution on github issue create for the bug.

###  Etcd-Main pod fails to come up, since backup-restore sidecar is reporting RevisionConsistencyCheckErr

#### Issue
- Etcd-main pod goes in `CrashLoopBackoff`.
- Etcd-backup-restore sidecar reports validation error with RevisionConsistencyCheckErr.

#### Environment
- Gardener version: 0.29.0+

#### Root Cause
- From version 0.29.0, Gardener uses shared backup bucket for storing etcd backups, replacing old logic of having single bucket per shoot as per [proposal](../proposals/02-backupinfra.md).
- Since there are very rare chances that etcd data directory will get corrupt, while doing this migration, to avoid etcd down time and implementation effort, we decided to switch directly from old bucket to new shared bucket without migrating old snapshot from old bucket to new bucket.
- In this case just for safety side we added sanity check in etcd-backup-restore sidecar of etcd-main pod, which checks if etcd data revision is greater than the last snapshot revision from old bucket.
- If above check fails mean there is surely some data corruption occurred with etcd, so etcd-backup-restore reports error and then etcd-main pod goes in `CrashLoopBackoff` creating etcd-main down alerts.

#### Action
1. Disable the Gardener reconciliation for Shoot by annotating it with `shoot.garden.sapcloud.io/ignore=true`
2. Scale down the etcd-main statefulset in seed cluster.
3. Find out the latest full snapshot and delta snapshot from old backup bucket. The old backup bucket name is same as the backupInfra resource associated with Shoot in Garden cluster.
4. Move them manually to new backup bucket.
5. Enable the Gardener reconciliation for shoot by removing annotation `shoot.garden.sapcloud.io/ignore=true`.

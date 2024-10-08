---
title: etcd
description: How Gardener uses the etcd key-value store
---

## etcd - Key-Value Store for Kubernetes

[etcd](https://etcd.io/) is a strongly consistent key-value store and the most prevalent choice for the Kubernetes
persistence layer. All API cluster objects like `Pod`s, `Deployment`s, `Secret`s, etc., are stored in `etcd`, which
makes it an essential part of a [Kubernetes control plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components).

## Garden or Shoot Cluster Persistence

Each garden or shoot cluster gets its very own persistence for the control plane.
It runs in the shoot namespace on the respective seed cluster (or in the `garden` namespace in the garden cluster, respectively).
Concretely, there are two etcd instances per shoot cluster, which the `kube-apiserver` is configured to use in the following way:

* `etcd-main`

A store that contains all "cluster critical" or "long-term" objects.
These object kinds are typically considered for a backup to prevent any data loss.

* `etcd-events`

A store that contains all `Event` objects (`events.k8s.io`) of a cluster.
`Events` usually have a short retention period and occur frequently, but are not essential for a disaster recovery.

The setup above prevents both, the critical `etcd-main` is not flooded by Kubernetes `Events`, as well as backup space is not occupied by non-critical data.
This separation saves time and resources.

## etcd Operator

Configuring, maintaining, and health-checking etcd is outsourced to a dedicated operator called [etcd Druid](https://github.com/gardener/etcd-druid/).
When a [`gardenlet`](gardenlet.md) reconciles a `Shoot` resource or a [`gardener-operator`](operator.md) reconciles a `Garden` resource, they manage an [`Etcd`](https://github.com/gardener/etcd-druid/blob/1d427e9167adac1476d1847c0e265c2c09d6bc62/config/samples/druid_v1alpha1_etcd.yaml) resource in the seed or garden cluster, containing necessary information (backup information, defragmentation schedule, resources, etc.).
`etcd-druid` needs to manage the lifecycle of the desired etcd instance (today `main` or `events`).
Likewise, when the `Shoot` or `Garden` is deleted, `gardenlet` or `gardener-operator` deletes the `Etcd` resources and [etcd Druid](https://github.com/gardener/etcd-druid/) takes care of cleaning up all related objects, e.g. the backing `StatefulSet`s.

## Backup

If `Seed`s specify backups for etcd ([example](../../example/50-seed.yaml)), then Gardener and the respective [provider extensions](../extensions/overview.md) are responsible for creating a bucket on the cloud provider's side (modelled through a [BackupBucket resource](../extensions/extension-resources/backupbucket.md)).
The bucket stores backups of `Shoot`s scheduled on that `Seed`.
Furthermore, Gardener creates a [BackupEntry](../extensions/extension-resources/backupentry.md), which subdivides the bucket and thus makes it possible to store backups of multiple shoot clusters.

How long backups are stored in the bucket after a shoot has been deleted depends on the configured _retention period_ in the `Seed` resource.
Please see this [example configuration](https://github.com/gardener/gardener/blob/849cd857d0d20e5dde26b9740ca2814603a56dfd/example/20-componentconfig-gardenlet.yaml#L20) for more information.

For `Garden`s specifying backups for etcd ([example](../../example/operator/20-garden.yaml)), the bucket must be pre-created externally and provided via the `Garden` specification.

Both etcd instances are configured to run with a special backup-restore _sidecar_.
It takes care about regularly backing up etcd data and restoring it in case of data loss (in the main etcd only).
The sidecar also performs defragmentation and other house-keeping tasks.
More information can be found in the [component's GitHub repository](https://github.com/gardener/etcd-backup-restore).

## Housekeeping

[etcd maintenance tasks](https://etcd.io/docs/v3.3/op-guide/maintenance/) must be performed from time to time in order to re-gain database storage and to ensure the system's reliability.
The [backup-restore](https://github.com/gardener/etcd-backup-restore) _sidecar_ takes care about this job as well.

For both `Shoot`s and `Garden`s, a random time **within the shoot's maintenance time** is chosen for scheduling these tasks.
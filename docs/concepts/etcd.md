# etcd - Key-Value Store for Kubernetes

[etcd](https://etcd.io/) is a strongly consistent key-value store and the most prevalent choice for the Kubernetes
persistence layer. All API cluster objects like `Pod`s, `Deployment`s, `Secret`s, etc. are stored in `etcd` which
makes it an essential part of a [Kubernetes control plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components).

## Shoot cluster persistence

Each shoot cluster gets its very own persistence for the control plane. It runs in the shoot namespace on the respective
seed cluster. Concretely, there are two etcd instances per shoot cluster which the `Kube-Apiserver` is configured
to use in the following way:

* etcd-main

A store that contains all "cluster critical" or "long-term" objects. These object kinds are typically considered
for a backup to prevent any data loss.

* etcd-events

A store that contains all `Event` objects (`events.k8s.io`) of a cluster. `Events` have usually a short retention
period, occur frequently but are not essential for a disaster recovery.

The setup above prevents both, the critical `etcd-main` is not flooded by Kubernetes `Events` as well as backup space is 
not occupied by non-critical data. This segmentation saves time and resources.

## etcd Operator

Configuring, maintaining and health-checking `etcd` is outsourced to a dedicated operator called [ETCD Druid](https://github.com/gardener/etcd-druid/).
When [Gardenlet](../concepts/gardenlet.md) reconciles a `Shoot` resource, it creates or updates an [Etcd](https://github.com/gardener/etcd-druid/blob/1d427e9167adac1476d1847c0e265c2c09d6bc62/config/samples/druid_v1alpha1_etcd.yaml)
resources in the seed cluster, containing necessary information (backup information, defragmentation schedule, resources, etc.) `etcd-druid`
needs to manage the lifecycle of the desired etcd instance (today `main` or `events`). Likewise, when the shoot is deleted,
Gardenlet deletes the `Etcd` resource and [ETCD Druid](https://github.com/gardener/etcd-druid/) takes care about cleaning up
all related objects, e.g. the backing `StatefulSet`.

## Autoscaling

Gardenlet maintains [HVPA](https://github.com/gardener/hvpa-controller/blob/master/config/samples/autoscaling_v1alpha1_hvpa.yaml)
objects for etcd `StatefulSet`s if the corresponding [feature gate](../deployment/feature_gates.md) is enabled. This enables
a vertical scaling for `etcd`. Downscaling is handled more pessimistic to prevent many subsequent `etcd` restarts. Thus,
for `production` clusters downscaling is deactivated and for all other clusters lower advertised requests/limits are only
applied during a shoot's maintenance time window.

## Backup

If `Seed`s specify backups for etcd ([example](https://github.com/gardener/gardener/blob/e9bf88a7a091a8cf8c495bef298bdada17a03c7f/example/50-seed.yaml#L19)),
then Gardener and the respective [provider extensions](../extensions/overview.md) are responsible for creating a bucket
on the cloud provider's side (modelled through [BackupBucket resource](../extensions/backupbucket.md)). The bucket stores
backups of shoots scheduled on that seed. Furthermore, Gardener creates a [BackupEntry](../extensions/backupentry.md)
which subdivides the bucket and thus makes it possible to store backups of multiple shoot clusters.

The `etcd-main` instance itself is configured to run with a special backup-restore _sidecar_. It takes care about regularly
backing up etcd data and restoring it in case of data loss. More information can be found on the component's GitHub
page https://github.com/gardener/etcd-backup-restore.

How long backups are stored in the bucket after a shoot has been deleted, depends on the configured _retention period_ in the
`Seed` resource. Please see this [example configuration](https://github.com/gardener/gardener/blob/849cd857d0d20e5dde26b9740ca2814603a56dfd/example/20-componentconfig-gardenlet.yaml#L20) for more information.

## Housekeeping

[etcd maintenance tasks](https://etcd.io/docs/v3.3/op-guide/maintenance/) must be performed from time to time in order
to re-gain database storage and to ensure the system's reliability. The [backup-restore](https://github.com/gardener/etcd-backup-restore)
_sidecar_ takes care about this job as well. Gardener chooses a random time **within the shoot's maintenance time** to
schedule these tasks.
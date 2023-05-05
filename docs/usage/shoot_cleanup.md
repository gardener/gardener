# Cleanup of Shoot Clusters in Deletion

When a shoot cluster is deleted then Gardener tries to gracefully remove most of the Kubernetes resources inside the cluster.
This is to prevent that any infrastructure or other artefacts remain after the shoot deletion.

The cleanup is performed in four steps.
Some resources are deleted with a grace period, and all resources are forcefully deleted (by removing blocking finalizers) after some time to not block the cluster deletion entirely.

**Cleanup steps:**

1. All `ValidatingWebhookConfiguration`s and `MutatingWebhookConfiguration`s are deleted with a `5m` grace period. Forceful finalization happens after `5m`.
1. All `APIService`s and `CustomResourceDefinition`s are deleted with a `5m` grace period. Forceful finalization happens after `1h`.
1. All `CronJob`s, `DaemonSet`s, `Deployment`s, `Ingress`s, `Job`s, `Pod`s, `ReplicaSet`s, `ReplicationController`s, `Service`s, `StatefulSet`s, `PersistentVolumeClaim`s are deleted with a `5m` grace period. Forceful finalization happens after `5m`.
   > If the `Shoot` is annotated with `shoot.gardener.cloud/skip-cleanup=true`, then only `Service`s and `PersistentVolumeClaim`s are considered.
1. All `VolumeSnapshot`s and `VolumeSnapshotContent`s are deleted with a `5m` grace period. Forceful finalization happens after `1h`.

It is possible to override the finalization grace periods via annotations on the `Shoot`:

- `shoot.gardener.cloud/cleanup-webhooks-finalize-grace-period-seconds` (for the resources handled in step 1)
- `shoot.gardener.cloud/cleanup-extended-apis-finalize-grace-period-seconds` (for the resources handled in step 2)
- `shoot.gardener.cloud/cleanup-kubernetes-resources-finalize-grace-period-seconds` (for the resources handled in step 3)

⚠️ If `"0"` is provided, then all resources are finalized immediately without waiting for any graceful deletion.
Please be aware that this might lead to orphaned infrastructure artefacts.

## Infrastructure Cleanup Wait Period

After all above cleanup steps have been performed and the `Infrastructure` extension resource has been deleted, the gardenlet waits for a certain duration to allow controllers to properly cleanup infrastructure resources.

By default, this duration is set to `5m`. Only after this time has passed, the shoot deletion flow continues with the entire tear-down of the remaining control plane components (including `kube-apiserver`s, etc.).

It is also possible to override this wait period via an annotations on the `Shoot`:

- `shoot.gardener.cloud/infrastructure-cleanup-wait-period-seconds`

> ℹ️️ All provided period values larger than the above mentioned defaults are ignored.

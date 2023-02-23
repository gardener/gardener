# Shoot Control Plane Migration "Bad Case" Scenario

The [migration flow](07-shoot-control-plane-migration.md#migration-workflow) described as part of [GEP-7](07-shoot-control-plane-migration.md) can only be executed if both the Garden cluster and source seed cluster are healthy, and the `gardenlet` in the source seed cluster can connect to the Garden cluster. In this case, the `gardenlet` can directly scale down the shoot's control plane in the source seed, after checking the `spec.seedName` field.

However, there might be situations in which the `gardenlet` in the source seed cluster can't connect to the Garden cluster and determine that `spec.seedName` has changed. Similarly, the connection to the seed `kube-apiserver` could also be broken. This might be caused by issues with the seed cluster itself. In other situations, the migration flow steps in the source seed might have started but might not be able to finish successfully. In all such cases, it should still be possible to migrate a shoot's control plane to a different seed, even though executing the migration flow steps in the source seed might not be possible. The potential "split brain" situation caused by having the shoot's control plane components attempting to reconcile the shoot resources in two different seeds must still be avoided, by ensuring that the shoot's control plane in the source seed is deactivated before it is activated in the destination seed.

The mechanisms and adaptations described below have been tested as part of a PoC prior to describing them here.

## Owner Election / Copying Snapshots

To achieve the goals outlined above, an "owner election" (or rather, "ownership passing") mechanism is introduced to ensure that the source and destination seeds are able to successfully negotiate a single "owner" during the migration. This mechanism is based on special *owner DNS records* that uniquely identify the seed that currently hosts the shoot's control plane ("owns" the shoot).

For example, for a shoot named `i500152-gcp` in project `dev` that uses an internal domain suffix `internal.dev.k8s.ondemand.com` and is scheduled on a seed with an identity `shoot--i500152--gcp2-0841c87f-8db9-4d04-a603-35570da6341f-sap-landscape-dev`, the owner DNS record is a TXT record with a domain name `owner.i500152-gcp.dev.internal.dev.k8s.ondemand.com` and a single value `shoot--i500152--gcp2-0841c87f-8db9-4d04-a603-35570da6341f-sap-landscape-dev`. The owner DNS record is created and maintained by reconciling an `owner` DNSRecord resource.

Unlike other extension resources, the `owner` DNSRecord resource is not reconciled every time the shoot is reconciled, but only when the resource is created. Therefore, the owner DNS record value (the owner ID) is updated only when the shoot is migrated to a different seed. For more information, see [Add handling of owner DNSRecord resources](https://github.com/gardener/gardener/pull/4307).

The owner DNS record domain name and owner ID are passed to components that need to perform ownership checks, such as the `backup-restore` container of the `etcd-main` StatefulSet, and all extension controllers. These components then check regularly whether the actual owner ID (the value of the record) matches the passed ID. If they don't, the ownership check is considered failed, which causes the special behavior described below.

> **Note:** A previous revision of this document proposed using "sync objects" written to and read from the backup container of the source seed as JSON files by the `etcd-backup-restore` processes in both seeds. With the introduction of owner DNS records such sync objects are no longer needed.

For the destination seed to actually become the owner, it needs to acquire the shoot's etcd data by copying the final full snapshot (and potentially also older snapshots) from the backup container of the source seed.

The mechanism to copy the snapshots and pass the ownership from the source to the destination seed consists of the following steps:

1. The reconciliation flow ("restore" phase) is triggered in the destination seed without first executing the migration flow in the source seed (or perhaps it was executed, but it failed, and its state is currently unknown).

2. The `owner` DNSRecord resource is created in the destination seed. As a result, the actual owner DNS record is updated with the destination seed ID. From this point, ownership checks by the `etcd-backup-restore` process and [extension controller watchdogs](#extension-controller-watchdogs) in the source seed will fail, which will cause the special behavior described below. 

3. An additional "source" backup entry referencing the source seed backup bucket is deployed to the Garden cluster and the destination seed and reconciled by the backup entry controller. As a result, a secret with the appropriate credentials for accessing the source seed backup container named `source-etcd-backup` is created in the destination seed. The normal backup entry (referencing the destination seed backup container) is also deployed and reconciled, as usual, resulting in the usual `etcd-backup` secret being created.

4. A special "copy" version of the `etcd-main` Etcd resource is deployed to the destination seed. In its `backup` section, this resource contains a `sourceStore` in addition to the usual `store`, which contains the parameters needed to use the source seed backup container, such as its name and the secret created in the previous step.

   ```yaml
   spec:
     backup:
       ...
       store:
         container: 408740b8-6491-415e-98e6-76e92e5956ac
         secretRef:
           name: etcd-backup
         ...
       sourceStore:
         container: d1435fea-cd5e-4d5b-a198-81f4025454ff
         secretRef:
           name: source-etcd-backup
         ...
   ```

5. The `etcd-druid` in the destination seed reconciles the above resource by deploying a `etcd-copy` Job that contains a single `backup-restore` container. It executes the newly introduced `copy` command of `etcd-backup-restore` that copies the snapshots from the source to the destination backup container.

6. Before starting the copy itself, the `etcd-backup-restore` process in the destination seed checks if a final full snapshot (a full snapshot marked as `final=true`) exists in the backup container. If such a snapshot is not found, it waits for it to appear in order to proceed. This waiting is up to a certain timeout that should be sufficient for a full snapshot to be taken; after this timeout has elapsed, it proceeds anyway, and the reconciliation flow continues from step 9. As described in [Handling Inability to Access the Backup Container](#handling-inability-to-access-the-backup-container) below, this is safe to do.

7. The `etcd-backup-restore` process in the source seed detects that the owner ID in the owner DNS record is different from the expected owner ID (because it was updated in step 2) and switches to a special "final snapshot" mode. In this mode the regular snapshotter is stopped, the readiness probe of the main `etcd` container starts returning 503, and one final full snapshot is taken. This snapshot is marked as `final=true` in order to ensure that it's only taken once, and in order to enable the `etcd-backup-restore` process in the destination seed to find it (see step 6).

   > **Note:** While testing our PoC, we noticed that simply making the readiness probe of the main `etcd` container fail doesn't terminate the existing open connections from `kube-apiserver` to `etcd`. For this to happen, either the `kube-apiserver` or the `etcd` process has to be restarted at least once. Therefore, when the snapshotter is stopped because an ownership change has been detected, the main `etcd` process is killed (using `SIGTERM` to allow graceful termination) to ensure that any open connections from `kube-apiserver` are terminated. For this to work, the 2 containers must [share the process namespace](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/).

8. Since the `kube-apiserver` process in the source seed is no longer able to connect to `etcd`, all shoot control plane controllers (`kube-controller-manager`, `kube-scheduler`, `machine-controller-manager`, etc.) and extension controllers reconciling shoot resources in the source seed that require a connection to the shoot in order to work start failing. All remaining extension controllers are prevented from reconciling shoot resources via the [watchdogs](#extension-controller-watchdogs) mechanism. At this point, the source seed has effectively lost its ownership of the shoot, and it is safe for the destination seed to assume the ownership.

9. After the `etcd-backup-restore` process in the destination seed detects that a final full snapshot exists, it copies all snapshots (or a subset of all snapshots) from the source to the destination backup container. When this is done, the Job finishes successfully, which signals to the reconciliation flow that the snapshots have been copied.

   > **Note:** To save time, only the final full snapshot taken in step 6, or a subset defined by some criteria, could be copied, instead of all snapshots.

10. The special "copy" version of the `etcd-main` Etcd resource is deleted from the source seed, and as a result the `etcd-copy` Job is also deleted by `etcd-druid`.

11. The additional "source" backup entry referencing the source seed backup container is deleted from the Garden cluster and the destination seed. As a result, its corresponding `source-etcd-backup` secret is also deleted from the destination seed.

12. From this point, the reconciliation flow proceeds as already described in [GEP-7](07-shoot-control-plane-migration.md). This is safe, since the source seed cluster is no longer able to interfere with the shoot.

## Handling Inability to Access the Backup Container

The mechanism described above assumes that the `etcd-backup-restore` process in the source seed is able to access its backup container in order to take snapshots. If this is not the case, but an ownership change was detected, the `etcd-backup-restore` process still sets the readiness probe status of the main `etcd` container to 503, and kills the main `etcd` process as described above to ensure that any open connections from `kube-apiserver` are terminated. This effectively deactivates the source seed control plane to ensure that the ownership of the shoot can be passed to a different seed.

Because of this, the `etcd-backup-restore` process in the destination seed responsible for copying the snapshots can avoid waiting forever for a final full snapshot to appear. Instead, after a certain timeout has elapsed, it can proceed with the copying. In this situation, whatever latest snapshot is found in the source backup container will be restored in the destination seed. The shoot is still migrated to a healthy seed at the cost of losing the etcd data that accumulated between the point in time when the connection to the source backup container was lost, and the point in time when the source seed cluster was deactivated.

When the connection to the backup container is restored in the source seed, a final full snapshot will be eventually taken. Depending on the stage of the restoration flow in the destination seed, this snapshot may be copied to the destination seed and restored, or it may simply be ignored since the snapshots have already been copied.

## Handling Inability to Resolve the Owner DNS Record

The situation when the owner DNS record cannot be resolved is treated similarly to a failed ownership check: the `etcd-backup-restore` process sets the readiness probe status of the main `etcd` container to 503, and kills the main `etcd` process as described above to ensure that any open connections from `kube-apiserver` are terminated, effectively deactivating the source seed control plane. The final full snapshot is not taken in this case to ensure that the control plane can be re-activated if needed.

When the owner DNS record can be resolved again, the following 2 situations are possible:

* If the source seed is still the owner of the shoot, the `etcd-backup-restore` process will set the readiness probe status of the main `etcd` container to 200, so `kube-apiserver` will be able to connect to `etcd` and the source seed control plane will be activated again.
* If the source seed is no longer the owner of the shoot, the etcd readiness probe will continue to fail, and the source seed control plane will remain inactive. In addition, the final full snapshot will be taken at this time, for the same reason as described in [Handling Inability to Access the Backup Container](#handling-inability-to-access-the-backup-container).

> **Note:** We expect that actual DNS outages are extremely unlikely. A more likely reason for an inability to resolve a DNS record could be network issues with the underlying infrastructure. In such cases, the shoot would usually not be usable / reachable anyway, so deactivating its control plane would not cause a worse outage. 

## Migration Flow Adaptations

Certain changes to the migration flow are needed in order to ensure that it is compatible with the [owner election](#owner-election--copying-snapshots) mechanism described above. Instead of taking a full snapshot of the source seed etcd, the flow deletes the owner DNS record by deleting the `owner` DNSRecord resource. This causes the ownership check by `etcd-backup-restore` to fail, and the final full snapshot to be eventually taken, so the migration flow waits for a final full snapshot to appear as the last step before deleting the shoot namespace in the source seed. This ensures that the reconciliation flow described above will find a final full snapshot waiting to be copied at step 6.

Checking for the final full snapshot is performed by calling the already existing `etcd-backup-restore` endpoint `snapshot/latest`. This is possible, since the `backup-restore` container is always running at this point.

After the final full snapshot has been taken, the readiness probe of the main `etcd` container starts failing, which means that if the migration flow is retried due to an error, it must skip the step that waits for `etcd-main` to become ready. To determine if this is the case, a check whether the final full snapshot has been taken or not is performed by calling the same `etcd-backup-restore` endpoint, e.g. `snapshot/latest`. This is possible if the `etcd-main` Etcd resource exists with non-zero replicas. Otherwise:

* If the resource doesn't exist, it must have been already deleted, so the final full snapshot n must have been already taken.
* If it exists with zero replicas, the shoot must be hibernated, and the migration flow must have never been executed (since it scales up etcd as one of its first steps), so the final full snapshot must not have been taken yet.

## Extension Controller Watchdogs

Some extension controllers will stop reconciling shoot resources after the connection to the shoot's `kube-apiserver` is lost. Others, most notably the infrastructure controller, will not be affected. Even though new shoot reconciliations won't be performed by the `gardenlet`, such extension controllers might be stuck in a retry loop triggered by a previous reconciliation, which may cause them to reconcile their resources after the `gardenlet` has already stopped reconciling the shoot. In addition, a reconciliation started when the seed still owned the shoot might take some time and therefore might still be running after the ownership has changed. To ensure that the source seed is completely deactivated, an additional safety mechanism is needed.

This mechanism should handle the following interesting cases:

* The `gardenlet` cannot connect to the Garden `kube-apiserver`. In this case, it cannot fetch shoots and therefore does not know if control plane migration has been triggered. Even though the `gardenlet` will not trigger new reconciliations, extension controllers could still attempt to reconcile their resources if they are stuck in a retry loop from a previous reconciliation, and already running reconciliations will not be stopped.
* The `gardenlet` cannot connect to the seed's `kube-apiserver`. In this case, the `gardenlet` knows if migration has been triggered, but it will not start shoot migration or reconciliation, as it will first check the seed conditions and try to update the `Cluster` resource, both of which will fail. Extension controllers could still be able to connect to the seed's `kube-apiserver` (if they are not running where the `gardenlet` is running), and similarly to the previous case, they could still attempt to reconcile their resources.
* The seed components (`etcd-druid`, extension controllers, etc) cannot connect to the seed's `kube-apiserver`. In this case, extension controllers would not be able to reconcile their resources, as they cannot fetch them from the seed's `kube-apiserver`. When the connection to the `kube-apiserver` comes back, the controllers might be stuck in a retry loop from a previous reconciliation, or the resources could still be annotated with `gardener.cloud/operation=reconcile`. This could lead to a race condition depending on who manages to `update` or `get` the resources first. If the `gardenlet` manages to update the resources before they are read by the extension controllers, they would be properly updated with `gardener.cloud/operation=migrate`. Otherwise, they would be reconciled as usual.

> **Note:** A previous revision of this document proposed using "cluster leases" as such an additional safety mechanism. With the introduction of owner DNS records, cluster leases are no longer needed.

The safety mechanism is based on *extension controller watchdogs*. These are simply additional goroutines that are started when a reconciliation is started by an extension controller. These goroutines perform an ownership check on a regular basis using the owner DNS record, similar to the check performed by the `etcd-backup-restore` process described above. If the check fails, the watchdog cancels the reconciliation context, which immediately aborts the reconciliation.

> **Note:** The `dns-external` extension controller is the only extension controller that neither needs the shoot's `kube-apiserver`, nor uses the watchdog mechanism described here. Therefore, this controller will continue reconciling `DNSEntry` resources even after the source seed has lost the ownership of the shoot. With the PoC, we manually delete the `DNSOwner` resources from the source seed cluster to prevent this from happening. Eventually, the `dns-external` controller should be adapted to use the owner DNS records to ensure that it disables itself after the seed has lost the ownership of the shoot. Changes in this direction have already been agreed and relevant PRs proposed.

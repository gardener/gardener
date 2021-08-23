# Shoot Control Plane Migration "Bad Case" Scenario

The [migration flow](07-shoot-control-plane-migration.md#migration-workflow) described as part of [GEP-7](07-shoot-control-plane-migration.md) can only be executed if both the Garden cluster and source seed cluster are healthy, and `gardenlet` in the source seed cluster can connect to the Garden cluster. In this case, `gardenlet` can directly scale down the shoot's control plane in the source seed, after checking the `spec.seedName` field.

However, there might be situations in which `gardenlet` in the source seed cluster can't connect to the Garden cluster and determine that `spec.seedName` has changed. Similarly, the connection to the seed `kube-apiserver` could also be broken. This might be caused by issues with the seed cluster itself. In other situations, the migration flow steps in the source seed might have started but might not be able to finish successfully. In all such cases, it should still be possible to migrate a shoot's control plane to a different seed, even though executing the migration flow steps in the source seed might not be possible. The potential "split brain" situation caused by having the shoot's control plane components attempting to reconcile the shoot resources in two different seeds must still be avoided, by ensuring that the shoot's control plane in the source seed is deactivated before it is activated in the destination seed.

The mechanisms and adaptations described below have been tested as part of a PoC prior to describing them here. This PoC also exposed a few weak or open points that should be resolved during the productization effort. They are mentioned as "notes" in the description below.

### Owner Election / Copying Snapshots

To achieve the goals outlined above, an "owner election" (or rather, "ownership passing") mechanism is introduced to ensure that the source and destination seeds are able to successfully negotiate a single "owner" during the migration. This mechanism is based on special "sync objects" (called "header files" in previous versions of this document) written to and read from the backup container of the source seed as JSON files by the `etcd-backup-restore` processes in both seeds.

For the destination seed to become the owner, it needs to acquire the shoot's etcd data by copying the latest full snapshot (and any incremental snapshots based on it) from the backup container of the source seed. This is achieved by introducing a `CopyOperation` sync object with the following structure:

```go
type CopyOperation struct {
	// Status is the current status of the copy operation, one of "Initial", "Ready", or "Done".
	Status OperationStatus `json:"status"`
}
```

The mechanism to copy the snapshots and pass the ownership from the source to the destination seed consists of the following steps:

1. The reconciliation flow ("restore" phase) is triggered in the destination seed without first executing the migration flow in the source seed (or perhaps it was executed, but it failed, and its state is currently unknown).

2. An additional "source" backup entry referencing the source seed backup bucket is deployed to the Garden cluster and the destination seed and reconciled by the backup entry controller. As a result, a secret with the appropriate credentials for accessing the source seed backup container named `source-etcd-backup` is created in the destination seed. The normal backup entry (referencing the destination seed backup container) is also deployed and reconciled, as usual, resulting in the usual `etcd-backup` secret being created.

3. A special "copy" version of the `etcd-main` Etcd resource is deployed to the destination seed. In its `backup` section, this resource contains a `sourceStore` in addition to the usual `store`, which contains the parameters needed to use the source seed backup container, such as its name and the secret created in the previous step.

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

4. The `etcd-druid` in the destination seed reconciles the above resource by deploying a `etcd-main` StatefulSet that doesn't contain the main `etcd` container at all. Instead, it only contains the `backup-restore` container that runs `etcd-backup-restore` configured to run in "copy" mode, which would only perform copying of the snapshots from the source to the destination backup container.

5. Before starting the copy itself, the `etcd-backup-restore` process in the destination seed creates the `CopyOperation` sync object mentioned above (if it doesn't already exist; see also [Migration Flow Adaptations](#migration-flow-adaptations)) with its status set to `Initial`. It then waits for the status of this sync object to become `Ready` in order to proceed. This waiting is up to a certain timeout that should be sufficient for a full snapshot to be taken; after this timeout has elapsed, it sets the status to `Ready` on its own, and the reconciliation flow continues from step 8, as it relies on the safety mechanism described in [Implicit Owner Election](#implicit-owner-election) below.

6. The `etcd-backup-restore` process in the source seed detects the `CopyOperation` sync object in status `Initial` and switches to a special "final snapshot" mode. In this mode the regular snapshotter is stopped, the readiness probe of the main `etcd` container starts returning 503, and one final full snapshot is taken. Finally, the status of the `CopyOperation` is set to `Ready`.

   **Note:** While testing our PoC, we noticed that simply making the readiness probe of the main `etcd` container fail doesn't terminate the existing open connections from `kube-apiserver` to `etcd`. For this to happen, either the `kube-apiserver` or the `etcd` process has to be restarted at least once. One way to achieve this is to make sure that every time the snapshotter is stopped (either because a copy operation was found or due to an error), the main `etcd` process is killed (using `SIGTERM` to allow graceful termination) to ensure that any open connections from `kube-apiserver` are terminated. For this to work, the 2 containers must [share the process namespace](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/).

7. Since the `kube-apiserver` process in the source seed is no longer able to connect to `etcd`, all shoot control plane controllers (`kube-controller-manager`, `kube-scheduler`, `machine-controller-manager`, etc.) and extension controllers reconciling shoot resources in the source seed that require a connection to the shoot in order to work start failing. All remaining extension controllers are prevented from reconciling shoot resources via the [cluster leases](#cluster-leases) mechanism, after the current lease expires. At this point, the source seed has effectively lost its ownership of the shoot, and it is safe for the destination seed to assume the ownership.

8. After the `etcd-backup-restore` process in the destination seed detects that status of the `CopyOperation` sync object has become `Ready`, it copies all snapshots from the source to the destination backup containers. When this is done, its readiness probe starts returning 200 to signal to the reconciliation flow that it has done its job. It also sets the status of the sync object to `Done` (for debugging purposes).

   **Note:** To save time, only the latest full snapshot (which should usually be the final full snapshot taken in step 6) and its incremental snapshots could be copied, instead of all snapshots.

9. The special "copy" version of the `etcd-main` Etcd resource is deleted from the source seed, and as a result the single-container `etcd-main` StatefulSet is also destroyed by `etcd-druid`.

10. The additional "source" backup entry referencing the source seed backup container is deleted from the Garden cluster and the destination seed. As a result, its corresponding `source-etcd-backup` secret is also deleted from the destination seed.

11. The next step in the reconciliation flow waits for the maximum time needed for the [cluster lease](#cluster-leases) in the source seed to expire (e.g. 2 minutes), before proceeding. This is a safety measure; the lease might have already expired long ago after the source seed lost its connection to the Garden cluster, but there is no way to be sure about it.

12. From this point, the reconciliation flow proceeds as already described above. This is safe, since the source seed cluster is no longer able to interfere with the shoot.

### Implicit Owner Election

The mechanism described above assumes that the `etcd-backup-restore` process in the source seed is able to access its backup container in order to read or write sync object and take snapshots. If this is not the case, the `etcd-backup-restore` process sets the readiness probe status of the main `etcd` container to 503 and also kills the main `etcd` process as described above to ensure that any open connections from `kube-apiserver` are terminated. This effectively deactivates the source seed cluster to ensure that the ownership of the shoot can be passed to a different seed.

When the connection to the backup container is restored, the following situations are possible depending on whether the `etcd-backup-restore` process finds a `CopyOperation` sync object in its backup container and its status:

* If there is no `CopyOperation` sync object, the `etcd-backup-restore` process sets the readiness probe status of the main `etcd` container back to 200 after it has started successfully and is ready to accept connections. This effectively reactivates the source seed cluster.
* If there is a `CopyOperation` sync object in status `Initial`, the standard owner election process described above continues from step 6.
* If there is a `CopyOperation` sync object in status `Ready` or `Done`, the `etcd-backup-restore` process continues returning 503 by the `etcd` readiness probe. The source seed cluster remains inactive in this case since the status of the `CopyOperation` indicates that it is no longer the owner.

Because of this, `etcd-backup-restore` process in the destination seed responsible for copying the snapshots can avoid waiting forever for the status of the `CopyOperation` to become `Ready`. Instead, after a certain timeout has elapsed, it can set the status to `Ready` on its own and proceed with whatever latest snapshot it finds in the source backup container, so the shoot could still be migrated to a healthy seed at the cost of losing the etcd data that accumulated between the point in time when the connection to the source backup container was lost, and the point in time when the source seed cluster was deactivated.

### Migration Flow Adaptations

Certain changes to the [migration flow](#migration-workflow) are needed in order to ensure that it is compatible with the [owner election](#owner-election--copying-snapshots) mechanism described above. Instead of taking a full snapshot of the source seed etcd, the flow initiates a copy operation by creating a `CopyOperation` sync object with status `Initial` in the source backup container, and waits for its status to become `Ready`, as the last step before deleting the shoot namespace in the source seed. This ensures that the reconciliation flow described above will find a `CopyOperation` in status `Ready` and a final full snapshot waiting to be copied, which means steps 6 and 7 could be skipped.

Creating the `CopyOperation` is performed by calling a special `etcd-backup-restore` endpoint, e.g. `copyop/initiate`. This is possible, since the `backup-restore` container is always running at this point.

After the copy operation has been initiated, the readiness probe of the main `etcd` container starts failing, which means that if the migration flow is retried due to an error it must skip the step that waits for `etcd-main` to become ready. To determine if this is the case, a check whether a copy operation has been initiated or not is performed by calling another special `etcd-backup-restore` endpoint, e.g. `copyop/status`. This is possible if the `etcd-main` Etcd resource exists with non-zero replicas. Otherwise:

* If the resource doesn't exist, it must have been already deleted, so the copy operation must have been already initiated.
* If it exists with zero replicas, the shoot must be hibernated, and the migration flow must have never been executed (since it scales up etcd as one of its first steps), so the copy operation must not have been initiated yet.

### Cluster Leases

Some extension controllers will stop reconciling shoot resources after the connection to the shoot's `kube-apiserver` is lost. Others, most notably the infastructure controller, will not be affected. Even though new shoot reconciliations won't be performed by `gardenlet`, such extension controllers might be stuck in a retry loop triggered by a previous reconciliation, which may cause them to reconcile their resources after `gardenlet` has already stopped reconciling the shoot. To ensure that the source seed is completely deactivated, an additional safety mechanism is needed.

This mechanism should handle the following interesting cases:

* `gardenlet` cannot connect to the Garden `kube-apiserver`. In this case it cannot fetch shoots and therefore does not know if control plane migration has been triggered. Even though `gardenlet` will not trigger new reconciliations, extension controllers could still attempt to reconcile their resources if they are stuck a retry loop from a previous reconciliation.
* `gardenlet` cannot connect to the seed's `kube-apiserver`. In this case `gardenlet` knows if migration has been triggered, but it will not start shoot migration or reconciliation as it will first check the seed conditions and try to update the `Cluster` resource, both of which will fail. Extension controllers could still be able to connect to the seed's `kube-apiserver` (if they are not running where `gardenlet` is running), and similarly to the previous case, they could still attempt to reconcile their resources.
* The seed components (`etcd-druid`, extension controllers, etc) cannot connect to the seed's `kube-apiserver`. In this case extension controllers would not be able to reconcile their resources as they cannot fetch them from the seed's `kube-apiserver`. When the connection to the `kube-apiserver` comes back, the controllers might be stuck in a retry loop from a previous reconciliation, or the resources could still be annotated with `gardener.cloud/operation=reconcile`. This could lead to a race condition depending on who manages to `update` or `get` the resources first. If `gardenlet` manages to update the resources before they are read by the extension controllers, they would be properly updated with `gardener.cloud/operation=migrate`. Otherwise, they would be reconciled as usual.

The safety mechanism is based on "cluster leases". Essentially, extension controllers must check if the seed has a valid "lease" for the shoot before attempting a reconciliation. The lease is just an additional timestamp field in the `Cluster` resource:

```go
// ClusterSpec is the spec for a Cluster resource.
type ClusterSpec struct {
	...
	// LeaseExpiration indicates when the lease of this cluster will expire
	LeaseExpiration metav1.MicroTime `json:"leaseExpiration"`
}
```

On a regular basis, e.g. every 2 minutes, and at each shoot reconciliation, `gardenlet` fetches the shoot from the Garden `kube-apiserver`, and if successful and the control plane migration hasn't been triggered it updates the `Cluster` resource with a renewed timestamp, e.g. 2 minutes from now. The extension controllers fetch this resource before each reconciliation and check if the lease has expired. If it hasn't, they reconcile as usual. If the lease has expired, they skip the reconciliation, since either the control plane migration has been triggered or `gardenlet` cannot reliably determine if the seed is still the owner.

**Note:** The `dns-external` extension controller is the only extension controller that neither needs the shoot's `kube-apiserver`, nor uses the `Cluster` resource for anything. Therefore, this controller will continue reconciling `DNSEntry` resources even after the source seed has lost the ownership of the shoot. In the PoC, we manually deleted the `DNSOwner` resources from the source seed cluster to prevent this from happening. Eventually, the `dns-external` controller should either use the "cluster leases" mechanism described here, or a different mechanism to ensure that it disables itself after the seed has lost the ownership of the shoot. 

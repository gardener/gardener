# Control Plane Migration

## Preconditions

To be able to use this feature you need to enable the feature gate `SeedChange` in your `gardener-apiserver` by adding the following command flag: `--feature-gates=SeedChange=true`.

Also, the involved Seeds need to have enabled BackupBuckets.

## ShootState

`ShootState` is an API resource which stores non-reconstructible state and data required to completely recreate a `Shoot`'s control plane on a new `Seed`.  The `ShootState` resource is created on `Shoot` creation in its `Project` namespace and the required state/data is persisted during `Shoot` creation or reconciliation.

## Shoot Control Plane Migration

Triggering the migration is done by changing the `Shoot`'s `.spec.seedName` to a `Seed` that differs from the `.status.seedName`, we call this `Seed` `"Destination Seed"`. This action can only be performed by an operator with necessary RBAC. If the Destination `Seed` does not have a backup and restore configuration, the change to `spec.seedName` is rejected. Additionally, this Seed must not be set for deletion and must be healthy.

If the `Shoot` has different `.spec.seedName` and `.status.seedName` a process is started to prepare the Control Plane for migration:

1. `.status.lastOperation` is changed to `Migrate`.
2. Kubernetes API Server is stopped and the extension resources are annotated with `gardener.cloud/operation=migrate`.
3. Full snapshot of the ETCD is created and terminating of the Control Plane in the `Source Seed` is initiated.

If the process is successful, we update the status of the `Shoot` by setting the `.status.seedName` to the null value. That way, a restoration is triggered in the `Destination Seed` and `.status.lastOperation` is changed to `Restore`. The control plane migration is completed when the `Restore` operation has completed successfully.

By default the shoot's etcd backups will continue to be uploaded to the `BackupBucket` of the `Source Seed` after control plane migration has finished. If you want the etcd backups to instead be uploaded to the `BackupBucket` of the `Destination Seed`, you have to enable the `CopyEtcdBackupsDuringControlPlaneMigration` feature gate on the `gardenlet` by adding the following command flag: `--feature-gates=CopyEtcdBackupsDuringControlPlaneMigration=true`. Note that this will also move the existing etcd backups to the `BackupBucket` of the `Destination Seed` during the control plane migration process.

## Triggering the migration

For controlplane migration, operators with necessary RBAC can use the [`shoots/binding`](../concepts/scheduler.md#shootsbinding-subresource) subresource to change the `.spec.seedName`, with the following commands:

```
export NAMESPACE=my-namespace
export SHOOT_NAME=my-shoot
export SERVER=cluster-server-address
curl -k --cert <path>/<to>/client.crt --key <path>/<to>/client.key -XPATCH -H "Accept: application/json" -H "Content-Type: application/merge-patch+json" --data '{"spec":{"seedName":"<destination-seed>"}}' https://127.0.0.1:54693/apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/binding | jq -r ".spec.seedName"
```
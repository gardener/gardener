# Control Plane Migration

## Prerequisites

Also, the involved Seeds need to have enabled `BackupBucket`s.

## ShootState

`ShootState` is an API resource which stores non-reconstructible state and data required to completely recreate a `Shoot`'s control plane on a new `Seed`.  The `ShootState` resource is created on `Shoot` creation in its `Project` namespace and the required state/data is persisted during `Shoot` creation or reconciliation.

## Shoot Control Plane Migration

Triggering the migration is done by changing the `Shoot`'s `.spec.seedName` to a `Seed` that differs from the `.status.seedName`, we call this `Seed` a `"Destination Seed"`. This action can only be performed by an operator with the necessary RBAC. If the Destination `Seed` does not have a backup and restore configuration, the change to `spec.seedName` is rejected. Additionally, this Seed must not be set for deletion and must be healthy.

If the `Shoot` has different `.spec.seedName` and `.status.seedName`, a process is started to prepare the Control Plane for migration:

1. `.status.lastOperation` is changed to `Migrate`.
2. Kubernetes API Server is stopped and the extension resources are annotated with `gardener.cloud/operation=migrate`.
3. Full snapshot of the ETCD is created and terminating of the Control Plane in the `Source Seed` is initiated.

If the process is successful, we update the status of the `Shoot` by setting the `.status.seedName` to the null value. That way, a restoration is triggered in the `Destination Seed` and `.status.lastOperation` is changed to `Restore`. The control plane migration is completed when the `Restore` operation has completed successfully.

The etcd backups will be copied over to the `BackupBucket` of the `Destination Seed` during control plane migration and any future backups will be uploaded there.

## Triggering the Migration

For controlplane migration, operators with the necessary RBAC can use the [`shoots/binding`](../concepts/scheduler.md#shootsbinding-subresource) subresource to change the `.spec.seedName`, with the following commands:

```
export NAMESPACE=my-namespace
export SHOOT_NAME=my-shoot
kubectl get --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME} | jq -c '.spec.seedName = "<destination-seed>"' | kubectl replace --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/binding -f - | jq -r '.spec.seedName'
```

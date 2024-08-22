# Control Plane Migration

## Prerequisites

The `Seed`s involved in the control plane migration must have backups enabled - their `.spec.backup` fields cannot be nil.

## ShootState

`ShootState` is an API resource which stores non-reconstructible state and data required to completely recreate a `Shoot`'s control plane on a new `Seed`.  The `ShootState` resource is created on `Shoot` creation in its `Project` namespace and the required state/data is persisted during `Shoot` creation or reconciliation.

## Shoot Control Plane Migration

Triggering the migration is done by changing the `Shoot`'s `.spec.seedName` to a `Seed` that differs from the `.status.seedName`, we call this `Seed` a `"Destination Seed"`. This action can only be performed by an operator (see [Triggering the Migration](#triggering-the-migration)). If the `Destination Seed` does not have a backup and restore configuration, the change to `spec.seedName` is rejected. Additionally, this Seed must not be set for deletion and must be healthy.

If the `Shoot` has different `.spec.seedName` and `.status.seedName`, a process is started to prepare the Control Plane for migration:

1. `.status.lastOperation` is changed to `Migrate`.
2. Kubernetes API Server is stopped and the extension resources are annotated with `gardener.cloud/operation=migrate`.
3. Full snapshot of the ETCD is created and terminating of the Control Plane in the `Source Seed` is initiated.

If the process is successful, we update the status of the `Shoot` by setting the `.status.seedName` to the null value. That way, a restoration is triggered in the `Destination Seed` and `.status.lastOperation` is changed to `Restore`. The control plane migration is completed when the `Restore` operation has completed successfully.

The etcd backups will be copied over to the `BackupBucket` of the `Destination Seed` during control plane migration and any future backups will be uploaded there.

## Triggering the Migration

For control plane migration, operators with the necessary RBAC can use the [`shoots/binding`](../concepts/scheduler.md#shootsbinding-subresource) subresource to change the `.spec.seedName`, with the following commands:

```bash
NAMESPACE=my-namespace
SHOOT_NAME=my-shoot
DEST_SEED_NAME=destination-seed

kubectl get --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME} | jq -c '.spec.seedName = "'${DEST_SEED_NAME}'"' | kubectl replace --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/binding -f - | jq -r '.spec.seedName'
```


> [!IMPORTANT]
> When migrating `Shoot`s to a `Destination Seed` with different provider type from the `Source Seed`, make sure of the following:
>
> Pods running in the `Destination Seed` must have network connectivity to the backup storage provider of the `Source Seed` so that etcd backups can be copied successfully. Otherwise, the `Restore` operation will get stuck at the `Waiting until etcd backups are copied` step. However, if you do end up in this case, you can still finish the control plane migration by following the [guide to manually copy etcd backups](#copying-etcd-backups-manually-during-the-restore-operation).
>
> The nodes of your `Shoot` cluster must have network connectivity to the `Shoot`'s `kube-apiserver` and the `vpn-seed-server` once they are migrated to the `Destination Seed`. Otherwise, the `Restore` operation will get stuck at the `Waiting until the Kubernetes API server can connect to the Shoot workers` step. However, if you do end up in this case and cannot allow network traffic from the nodes to the `Shoot`'s control plane, you can annotate the `Shoot` with the `shoot.gardener.cloud/skip-readiness` annotation so that the `Restore` operation finishes, and then use the [`shoots/binding`](../concepts/scheduler.md#shootsbinding-subresource) subresource to migrate the control plane back to the `Source Seed`.


## Copying ETCD Backups Manually During the `Restore` Operation

Following is a workaround that can be used to copy etcd backups manually in situations where a `Shoot`'s control plane has been moved to a `Destination Seed` and the pods running in it lack network connectivity to the `Source Seed`'s storage provider:

1. Follow the instructions in the [`etcd-backup-restore` getting started documentation](https://github.com/gardener/etcd-backup-restore/blob/master/docs/deployment/getting_started.md#getting-started) on how to run the `etcdbrctl` command locally or in a container.
1. Follow the instructions in the [passing-credentials guide](https://github.com/gardener/etcd-backup-restore/blob/master/docs/deployment/getting_started.md#passing-credentials) on how to set up the required credentials for the copy operation depending on the storage providers for which you want to perform it.
1. Use the `etcdbrctl copy` command to copy the backups by following the instructions in the [`etcdbrctl copy` guide](https://github.com/gardener/etcd-backup-restore/blob/master/docs/deployment/getting_started.md#etcdbrctl-copy)
1. After you have successfully copied the etcd backups, wait for the `EtcdCopyBackupsTask` custom resource to be created in the `Shoot`'s control plane on the `Destination Seed`, if it does not already exist. Afterwards, mark it as successful by patching it using the following command:
    ```
    SHOOT_NAME=my-shoot
    PROJECT_NAME=my-project

    kubectl patch -n shoot--${PROJECT_NAME}--${SHOOT_NAME} etcdcopybackupstask ${SHOOT_NAME} --subresource status --type merge -p "{\"status\":{\"conditions\":[{\"type\":\"Succeeded\",\"status\":\"True\",\"reason\":\"manual copy successful\",\"message\":\"manual copy successful\",\"lastTransitionTime\":\"$(date -Iseconds)\",\"lastUpdateTime\":\"$(date -Iseconds)\"}]}}"
    ```
1. After the `main-etcd` becomes `Ready`, and the `source-etcd-backup` secret is deleted from the `Shoot`'s control plane, remove the finalizer on the source `extensions.gardener.cloud/v1alpha1.BackupEntry` in the `Destination Seed` so that it can be deleted successfully (the resource name uses the following format: `source-shoot--<project-name>--<shoot-name>--<uid>`). This is necessary as the `Destination Seed` will not have network connectivity to the `Source Seed`'s storage provider and the deletion will fail.
1. Once the control plane migration has finished successfully, make sure to manually clean up the source backup directory in the `Source Seed`'s storage provider.
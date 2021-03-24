# Contract: `BackupEntry` resource

The Gardener project features a sub-project called [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) to take periodic backups of etcd backing Shoot clusters. It demands the bucket (or its equivalent in different object store providers) access credentials to be created and configured externally with appropriate credentials. The `BackupEntry` resource takes this responsibility in Gardener to provide this information by creating a secret specific to the component. Said that, the core motivation for introducing this resource was to support retention of backups post deletion of `Shoot`. The etcd-backup-restore components takes responsibility of garbage collecting old backups out of the defined period. Once a shoot is deleted, we need to persist the backups for few days. Hence, Gardener uses the `BackupEntry` resource for this housekeeping work post deletion of a `Shoot`. The `BackupEntry` resource is responsible for shoot specific prefix under referred bucket.

Before introducing the `BackupEntry` extension resource Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [here](https://github.com/gardener/gardener/tree/0.27.0/charts/seed-terraformer/charts/aws-backup)).
Now, Gardener commissions an external, provider-specific controller to take over this task. You can also refer to backupInfra proposal documentation to get idea about how the transition was done and understand the resource in broader scope.

## What is the lifespan of `BackupEntry`?

The bucket associated with `BackupEntry` will be created at using `BackupBucket` resource. The `BackupEntry` resource will be created as a part of a `Shoot` creation. But resource might continue to exist post deletion of a `Shoot` (see [this](../concepts/gardenlet.md#backupentry-controller) for more details).

## What needs to be implemented to support a new infrastructure provider?

As part of the shoot flow Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupEntry
metadata:
  name: shoot--foo--bar
spec:
  type: azure
  providerConfig:
    <some-optional-provider-specific-backup-bucket-configuration>
  backupBucketProviderStatus:
    <some-optional-provider-specific-backup-bucket-status>
  region: eu-west-1
  bucketName: foo
  secretRef:
    name: backupprovider
    namespace: shoot--foo--bar
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used to create the needed resources. This provider secret will be propagated from `BackupBucket` resource by Shoot controller.

Your controller is supposed to create the `etcd-backup` secret in control-plane namespace of a shoot. This secret is supposed to be used by Gardener or eventually the etcd-backup-restore component to backup the etcd. The controller implementation should cleanup the objects created under shoot specific prefix in bucket equivalent to name of `BackupEntry` resource.

In order to support a new infrastructure provider you need to write a controller that watches all `BackupBucket`s with `.spec.type=<my-provider-name>`. You can take a look at the below referenced example implementation for the Azure provider.

## References and additional resources

* [`BackupEntry` API Reference](https://gardener.cloud/api-reference/extensions/#extensions.gardener.cloud/v1alpha1.BackupBucket)
* [Exemplary implementation for the Azure provider](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/backupentry)
* [`BackupBucket` resource documentation](./backupbucket.md)
* [Shared bucket proposal](../proposals/02-backupinfra.md)
* [Gardener-controller-manager-component-config API specification](https://github.com/gardener/gardener/blob/master/pkg/controllermanager/apis/config/types.go#L101-#L107)

---
title: BackupEntry
---

# Contract: `BackupEntry` Resource

The Gardener project features a sub-project called [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) to take periodic backups of etcd backing Shoot clusters. It demands the bucket (or its equivalent in different object store providers) access credentials to be created and configured externally with appropriate credentials. The `BackupEntry` resource takes this responsibility in Gardener to provide this information by creating a secret specific to the component.

That being said, the core motivation for introducing this resource was to support retention of backups post deletion of `Shoot`. The etcd-backup-restore components take responsibility of garbage collecting old backups out of the defined period. Once a shoot is deleted, we need to persist the backups for few days. Hence, Gardener uses the `BackupEntry` resource for this housekeeping work post deletion of a `Shoot`. The `BackupEntry` resource is responsible for shoot specific prefix under referred bucket.

Before introducing the `BackupEntry` extension resource, Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [AWS Backup](https://github.com/gardener/gardener/tree/0.27.0/charts/seed-terraformer/charts/aws-backup)).
Now, Gardener commissions an external, provider-specific controller to take over this task. You can also refer to [backupInfra proposal documentation](../../proposals/02-backupinfra.md) to get idea about how the transition was done and understand the resource in broader scope.

## What Is the Lifespan of a `BackupEntry`?

The bucket associated with `BackupEntry` will be created by using a `BackupBucket` resource. The `BackupEntry` resource will be created as a part of the `Shoot` creation. But resources might continue to exist post deletion of a `Shoot` (see [gardenlet](../../concepts/gardenlet.md#backupentry-controller) for more details).

## What Needs to be Implemented to Support a New Infrastructure Provider?

As part of the shoot flow, Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

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

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used to create the needed resources. This provider secret will be propagated from the `BackupBucket` resource by the shoot controller.

Your controller is supposed to create the `etcd-backup` secret in the control plane namespace of a shoot. This secret is supposed to be used by Gardener or eventually by the etcd-backup-restore component to backup the etcd. The controller implementation should clean up the objects created under the shoot specific prefix in the bucket equivalent to the name of the `BackupEntry` resource.

In order to support a new infrastructure provider, you need to write a controller that watches all the `BackupBucket`s with `.spec.type=<my-provider-name>`. You can take a look at the below referenced example implementation for the Azure provider.

## References and Additional Resources

* [`BackupEntry` API Reference](../../api-reference/extensions.md#backupbucket)
* [Exemplary Implementation for the Azure Provider](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/backupentry)
* [`BackupBucket` Resource Documentation](./backupbucket.md)
* [Shared Bucket Proposal](../../proposals/02-backupinfra.md)
* [Gardener-controller-manager-component-config API Specification](../../../pkg/controllermanager/apis/config/v1alpha1/types.go#L101-#L107)

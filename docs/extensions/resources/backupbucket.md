---
title: BackupBucket
---

# Contract: `BackupBucket` Resource

The Gardener project features a sub-project called [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) to take periodic backups of etcd backing Shoot clusters. It demands the bucket (or its equivalent in different object store providers) to be created and configured externally with appropriate credentials. The `BackupBucket` resource takes this responsibility in Gardener.

Before introducing the `BackupBucket` extension resource, Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [AWS Backup](https://github.com/gardener/gardener/tree/0.27.0/charts/seed-terraformer/charts/aws-backup)).
Now, Gardener commissions an external, provider-specific controller to take over this task. You can also refer to [backupInfra proposal documentation](../../proposals/02-backupinfra.md) to get an idea about how the transition was done and understand the resource in a broader scope.

## What Is the Scope of a Bucket?

A bucket will be provisioned per `Seed`. So, a backup of every `Shoot` created on that `Seed` will be stored under a different shoot specific prefix under the bucket.
For the backup of the `Shoot` rescheduled on different `Seed`, it will continue to use the same bucket.

## What Is the Lifespan of a `BackupBucket`?

The bucket associated with `BackupBucket` will be created at the creation of the `Seed`. And as per current implementation, it will also be deleted on deletion of the `Seed`, if there isn't any `BackupEntry` resource associated with it.

In the future, we plan to introduce a schedule for `BackupBucket` - the deletion logic for the `BackupBucket` resource, which will reschedule it on different available `Seed`s on deletion or failure of a health check for the currently associated `seed`. In that case, the `BackupBucket` will be deleted only if there isn't any schedulable `Seed` available and there isn't any associated `BackupEntry` resource.

## What Needs to Be Implemented to Support a New Infrastructure Provider?

As part of the seed flow, Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupBucket
metadata:
  name: foo
spec:
  type: azure
  providerConfig:
    <some-optional-provider-specific-backupbucket-configuration>
  region: eu-west-1
  secretRef:
    name: backupprovider
    namespace: shoot--foo--bar
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used to create the needed resources. This provider secret will be configured by the Gardener operator in the `Seed` resource and propagated over there by the seed controller.

After your controller has created the required bucket, if required, it generates the secret to access the objects in the bucket and put a reference to it in `status.generatedSecretRef`.
The secret should be created in the namespace specified in the `backupbucket.extensions.gardener.cloud/generated-secret-namespace` annotation.
In case the annotation is not present, the `garden` namespace should be used.
This secret is supposed to be used by Gardener, or eventually a `BackupEntry` resource and etcd-backup-restore component, for backing up the etcd.

In order to support a new infrastructure provider, you need to write a controller that watches all `BackupBucket`s with `.spec.type=<my-provider-name>`. You can take a look at the below referenced example implementation for the Azure provider.

## References and Additional Resources

* [`BackupBucket` API Reference](../../api-reference/extensions.md#backupbucket)
* [Exemplary Implementation for the Azure Provider](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/backupbucket)
* [`BackupEntry` Resource Documentation](./backupentry.md)
* [Shared Bucket Proposal](../../proposals/02-backupinfra.md)

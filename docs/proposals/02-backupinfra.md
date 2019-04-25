# Backup Infrastructure CRD and Controller Redesign

## Goal

- As an operator, I would like to efficiently use the backup bucket for multiple clusters, thereby limiting the total number of buckets required.
- As an operator, I would like to use different cloud provider for backup bucket provisioning other than cloud provider used for seed infrastructure.
- Have seed independent backups, so that we can easily migrate a shoot from one seed to another.
-	Execute the backup operations (including bucket creation and deletion) from a seed, because network connectivity may only be ensured from the seeds (not necessarily from the garden cluster).
-	Preserve the garden cluster as source of truth (no information is missing in the garden cluster to reconstruct the state of the backups even if seed and shoots are lost completely).
-	Do not violate the infrastructure limits in regards to blob store limits/quotas.

## Motivation

Currently, every shoot cluster has its own etcd backup bucket with a centrally configured retention period. With the growing number of clusters, we are soon running out of the [quota limits of buckets on the cloud provider](https://gist.github.com/swapnilgm/5c4d5506811e63c32ab3d73c4171d30f). Moreover, even if the clusters are deleted, the backup buckets do exist, for a configured period of retention. Hence, there is need of minimizing the total count of buckets.

In addition, currently we use seed infrastructure credentials to provision the bucket for etcd backups. This results in binding backup bucket provider to seed infrastructure provider.

## Terminology

* __Bucket__ : It is equivalent to s3 bucket, abs container, gcs bucket, swift container, alicloud bucket
* __Object__ : It is equivalent s3 object, abs blob, gcs object, swift object, alicloud object,  snapshot/backup of etcd on object store.
* __Directory__ : As such there is no concept of directory in object store but usually the use directory as `/` separate common prefix for set of objects. Alternatively they use term folder for same.
* __deletionGracePeriod__: This means grace period or retention period for which backups will be persisted post deletion of shoot.

## Current Spec:
```YAML
#BackupInfra spec
Kind: BackupInfrastructure
Spec:
    seed: seedName
    shootUID : shoot.status.uid
```

## Current naming conventions
|||
|--|--|
|SeedNamespace : |Shoot--projectname--shootname|
|seed: |seedname|
|ShootUID : |shoot.status.UID|
|BackupInfraname:| seednamespce+sha(uid)[:5]|
|Backup-bucket-name: |BackupInfraName|
|BackupNamespace:| backup--BackupInfraName|

## Proposal

Considering [Gardener extension proposal] in mind, the backup infrastructure controller can be divided in two parts. There will be basically four backup infrastructure related CRD's. Two on the garden apiserver. And two on the seed cluster. Before going into to workflow, let's just first have look at the CRD.

### CRD on Garden cluster
Just to give brief before going into the details, we will be sticking to the fact that Garden apiserver is always source of truth. Since backupInfra will be maintained post deletion of shoot, the info regarding this should always come from garden apiserver, we will continue to have BackupInfra resource on garden apiserver with some modifications.

```yaml
apiVersion: garden.cloud/v1alpha1
kind: BackupBucket
metadata:
  name: packet-random[:5]
  # No namespace needed. This will be cluster scope resource.
  ownerReferences:
  - kind: CloudProfile
    name: packet
spec:
  provider: aws
  region: eu-west-1
  secretRef: # Required for root
    name: backup-operator-aws
    namespace: backup-garden
status:
  lastOperation: ...
  observedGeneration: ...
  seed: ...
```

```yaml
apiVersion: garden.cloud/v1alpha1
kind: BackupEntry
metadata:
  name: shoot--dev--example--3ef42 # Naming convention explained before
  namespace: garden-dev
  ownerReferences:
  - apiVersion: garden.sapcloud.io/v1beta1
    blockOwnerDeletion: false
    controller: true
    kind: Shoot
    name: example
    uid: 19a9538b-5058-11e9-b5a6-5e696cab3bc8
spec:
  shootUID: 19a9538b-5058-11e9-b5a6-5e696cab3bc8 # Just for reference to find back associated shoot.
  # Following section comes from cloudProfile or seed yaml based on granularity decision.
  provider: aws
  region: eu-west-1
  secretRef: # Required for root
    name: backup-operator-aws
    namespace: backup-garden
status:
  lastOperation: ...
  observedGeneration: ...
  seed: ...
```

### CRD on Seed cluster
Considering the extension proposal, we want individual component to be handled by controller inside seed cluster. We will have Backup related resource in registered seed cluster as well.

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupBucket
metadata:
  name: packet-random[:5]
  # No namespace need. This will be cluster scope resource
spec:
  type: aws
  region: eu-west-1
  secretRef:
    name: backup-operator-aws
    namespace: backup-garden
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
```

There are two points for introducing BackupEntry resource.
1. Cloud provider specific code goes completely in seed cluster.
2. Network issue is also handled by moving deletion part to backup-extension-controller in seed cluster.

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupEntry
metadata:
  name: shoot--dev--example--3ef42 # Naming convention explained later
  namespace: backup-garden
spec:
  type: aws
  region: eu-west-1
  secretRef: # Required for root
    name: backup-operator-aws
    namespace: backup-garden
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
```

### Workflow

- Gardener administrator will configure the cloudProfile with backup infra credentials and provider config as follows.

```yaml
# CloudProfile.yaml:
Spec:
    backup:
        provider: aws
        region: eu-qest-1
        secretRef:
            name: backup-operator-aws
            namespace: garden
```
Here CloudProfileController will interpret this spec as follows:
- If `spec.backup.region` is not nil,
  - Then respect it, i.e. use the provider and unique region field mentioned there for BackupBucket.
  - Here Preferably, `spec.backup.region` field will be unique, since for cross provider, it doesn’t make much sense. Since region name will be different for different providers.
- Otherwise, spec.backup.region is nil then,
  - If same provider case i.e. spec.backup.provider = spec.(type-of-provider)  or nil,
    - Then, for each region from `spec.(type-of-provider).constraints.regions` create a `BackupBucket` instance. This can be done lazily i.e. create `BackupBucket` instance for region only if some seed actually spawned in the region has been registered. This will avoid creating IaaS bucket even if no seed is registered in that region, but region is listed in `cloudprofile`.
    - Shoot controller will choose backup container as per the seed region. (With shoot control plane migration also, seed’s availibility zone might change but the region will be remaining same as per current scope.)
  - Otherwise cross provider case i.e. spec.backup.provider != spec.(type-of-provider)
    - Report validation error: Since, for example,  we can’t expect `spec.backup.provider` = `aws`  to support region in, `spec.packet.constraint.region`. Where type-of-provider is `packet`

Following diagram represent overall flow in details:

![sequence-diagram](Backup-Infrastructure-Provisioning-sequence-diagram.svg)

#### Reconciliation

Reconciliation on backup entry in seed cluster mostly comes in picture at the time of deletion. But we can add initialization steps like creation of [directory](#terminology) specific to shoot in backup bucket. We can simply create BackupEntry at the time of shoot deletion as well.

#### Deletion
- On shoot deletion, the BackupEntry instance i.e. shoot specific instance will get deletion timestamp because of ownerReference.
- If `deletionGracePeriod` configured in GCM component configuration is expired, BackupInfrastructure Controller will delete the backup folder associated with it from backup object store.
- Finally, it will remove the `finalizer` from backupEntry instance.

### Alternative

![sequence-diagram](Backup-Infrastructure-Provisioning-with-deletion-job.svg)

## Discussion points / variations
### Manual vs dynamic bucket creation
- As per limit observed on different cloud providers, we can have single bucket for backups on one cloud providers. So, we could avoid the little complexity introduced in above approach by pre-provisioning buckets as a part of landscape setup. But there won't be anybody to detect bucket existence and its reconciliation. Ideally this should be avoided.

- Another thing we can have is, we can let administrator register the pool of root backup infra resource and let the controller schedule backup on one of this.

- One more variation here could be to create bucket dynamically per hash of shoot UID.


### SDK vs Terraform
Initial reason for going for terraform script is its stability and the provided parallelism/concurrency in resource creation. For backup infrastructure, Terraform scripts are very minimal right now. Its simply have bucket creation script. With shared bucket logic, if possible we might want to isolate access at [directory](#terminology) level but again its additional one call. So, we will prefer switching to SDK for all object store operations.

### Limiting the number of shoots per bucket
Again as per limit observed on different cloud providers, we can have single bucket for backups on one cloud providers. But if we want to limit the number of shoots associated with bucket, we can have central map of configuration in `gardener-controller-component-configuration.yaml`.
Where we will mark supported count of shoots per cloud provider. Most probable space could be,
`controller.backupInfrastructures.quota`. If limit is reached we can create new `BucketBucket` instance.

e.g.
```yaml
apiVersion: controllermanager.config.gardener.cloud/v1alpha1
kind: ControllerManagerConfiguration
controllers:
  backupInfrastructure:
    quota:
      - provider: aws
        limit: 100 # Number mentioned here are random, just for example purpose.
      - provider: azure
        limit: 80
      - provider: openstack
        limit: 100
      ...
```

## Backward compatibility
### Migration
- Create shoot specific folder.
- Transfer old objects.
- Create manifest of objects on new bucket
    - Each entry will have status: None,Copied, NotFound.
    - Copy objects one by one.
- Scale down etcd-main with old config. :warning: Cluster down time
- Copy remaining objects
- Scale up etcd-main with new config.
- Destroy Old bucket and old backup namespace. It can be immediate or preferrably __lazy__ deletion.

![backup-migration-sequence-diagram](./Backup-Migration.svg)

### Legacy Mode alternative
- If Backup namespace present in seed cluster, then follow the legacy approach.
- i.e. reconcile creation/existence of shoot specific bucket and backup namespace.
- If backup namespace is not created, use shared bucket.
- __Limitation__ Never know when the existing cluster will be deleted, and hence, it might be little difficult to maintain with next release of gardener. This might look simple and straight-forward for now but may become pain point in future, if in worst case, because of some new use cases or refactoring, we have to change the design again. Also, even after multiple garden release we won't be able to remove deprecated existing BackupInfrastructure CRD

<!--
## Extension
 :ballot_box_with_check: _TODO:_ Out-of-tree object store interface library.
-->

### References
* [Gardener extension proposal]
* [Cloud providers object store limit comparison](https://gist.github.com/swapnilgm/5c4d5506811e63c32ab3d73c4171d30f)


[references]: #references
[Gardener extension proposal]: https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md#backup-infrastructure-provisioning

# Manual cleanup of Shoot cluster and associated kubernetes and infra resources

Though gardener takes care of graceful deletion of all hosted resources, even for non-healthy or cluster with creation/reconciliation error; sometimes, may be because of external action or some unknown issue, specifically in case something goes wrong with the local setup for development, we have to delete the cluster and cleanup all the resource manually, to save cost and not have issue while creating new shoot mostly with same name on same landscape. This document provides the list of resources to be cleaned up manually to remove the complete trace of the shoot cluster.

## Terminology

- __ShootTechnicalID__ : The value of `shoot.status.technicalID` in shoot resource YAML, typically equals to _"shoot--[project-name]--[shoot-name]"_
- __ShootUid__ : The value of `shoot.status.uid` in shoot resource YAML.

## Kubernetes resource
As a part of shoot creation, gardener creates some kubernetes resource on Seed and virtual-garden cluster. We have to clean this up so that gardener doesn't waste cpu cycles in reconciling them.

### On virtual-garden cluster:
- Delete the `shoot.core.gardener.cloud` resource from project namespace.
- Delete the `backupentry.core.gardener.cloud` resource from project namespace.

### On seed cluster:
- Delete the namespace with name equivalent to `ShootTechnicalID` from seed cluster on which shoot was hosted.
    - Remove the finalizers on the namespace scoped resources from that namespace to let namespace deletion succeed and not stuck in terminating state.
- Delete the associated cluster scoped resource `backupentry.extension.gardener.cloud` with name `ShootTechnicalID--ShootUid` hosted on the referred seed along with the referred `secret` resources.

## Cloud provider resource

As a part of Shoot hosting, gardener create different cloud providers resources like vpc, vm, object in objet store, etc. Missing out on cleaning up some of these resource may end up costing you unnecessary. These resources are spread up across max three accounts one for shoot cloud provide account, dns account, and object store account though usually all might be same for development purpose. Please follow the doc to cleanup all cloud provider resources.

### Shoot cloud provider account

Gardener as a part of shoot reconciliation create the different resource in end-users account. i.e the shoot owners account.

#### AWS
- Delete the VPC with name `ShootTechnicalID`.
- Delete the IAM roles related to shoot with name:
    - `ShootTechnicalID-bastions`
    - `ShootTechnicalID-nodes`

#### Azure
- Delete the resource group with the name `ShootTechnicalID-ShootUID[:5]`.

#### GCP
- Delete the VPC network with the name `ShootTechnicalID`.

#### Openstack
- Delete the network with the name `ShootTechnicalID` and its dependencies.


### Cleaning up Backups

Gardener regularly take the backup of etcd backing shoot cluster.
While purging shoot one has to consider cleaning up these backups as well.
You can get the backup provider account

#### AWS
- Go to AWS S3 web-console and login to configured backup provider account. This is the same account which you have configured under `seed.spec.backup` section.
- Go to the bucket used to shoot backup. You can find the bucket name from the `backupentry.core.gardener.cloud` object with name `ShootTechnicalID--ShootUID` at `.spec.bucketname`.
- Delete all the objects under prefix `ShootTechnicalID--ShootUid` from that bucket.

#### Azure
- Go to Microsoft Azure web-console and login to configured backup provider subscription. This is the same account which you have configured under `seed.spec.backup` section.
- Go to the bucket used for shoot backup. You can find the bucket name from the `backupentry.core.gardener.cloud` object with name `ShootTechnicalID--ShootUID` at `.spec.bucketname`.
- Delete all the objects under prefix `ShootTechnicalID--ShootUid` from that bucket.

#### GCP

- Go to Google cloud platform web-console and login to configured backup provider account. This is the same account which you have configured under `seed.spec.backup` section.
- Go to the bucket used for shoot backup. You can find the bucket name from the `backupentry.core.gardener.cloud` object with name `ShootTechnicalID--ShootUID` at `.spec.bucketname`.
- Delete all the objects under prefix `ShootTechnicalID--ShootUid` from that bucket.

#### Openstack

- Go to Openstack Horizon console and login to configured backup provider account. This is the same account which you have configured under `seed.spec.backup` section.
- Go to the bucket used for shoot backup. You can find the bucket name from the `backupentry.core.gardener.cloud` object with name `ShootTechnicalID--ShootUID` at `.spec.bucketname`.
- Delete all the objects under prefix `ShootTechnicalID--ShootUid` from that bucket.

### Cleaning up DNS

Gardener create multiple DNS entries for making different `service` resources available on the internet. These can be spread across two different accounts. One used for internal purpose which is configured globally by operator. And one for external purpose, accessible to shoot owner.

#### Internal DNS records
- While setting up gardener it, creates DNS entry specific to shoot under internal hosted DNS zone.
- You can find the associated DNS provider account details and hosted zone details under secret in `garden` namespace on virtual-garden label with  `garden.sapcloud.io/role: internal-domain`. Might change to `garden.cloud/role: internal-domain` in future.
    - You can use following command to get the DNS provider details:
    ```console
    kubectl -n garden get secret -l "garden.sapcloud.io/role=internal-domain" -o yaml
    ```
    - The annotation on secret `dns.gardener.cloud/provider` will mention the DNS provider name.
    - Secret data will give you the credentials for accessing the DNS provider account.
    - The annotation on secret `dns.gardener.cloud/domain` will provide the configured internal domain name.
- Go to DNS provider web console and cleanup the all DNS entries ending with `.<shoot-name>.<project-name>.internal.<internal-domain-name>`. If `internal.` is a part of internal-domain-name then just ignore it in avoid duplication in above pattern.

#### External DNS records
- External domain entries are accessible to Shoot owner and used to provision the DNS entries for LoadBalancer/ingress services on Shoot.
- Domain and provider for this can be either shoot specific or default domain configured globally.
- Shoot specific domain details can be found under `shoot.spec.dns.`
    - `shoot.spec.dns.domain` mentions the external domain name.
    - `shoot.spec.dns.provider` has the DNS providers name and secret reference under which access credentials will be stored.
- If shoot specific domain not configured then gardener uses the Default domain which is configured globally.
    - You can find the default DNS provider account details and domain details under secret in `garden` namespace on virtual-garden label with  `garden.sapcloud.io/role: default-domain`. Might change to `garden.cloud/role: internal-domain` in future.
    - You can use following command to get the DNS provider details:
    ```console
    kubectl -n garden get secret -l "garden.sapcloud.io/role=internal-domain" -o yaml
    ```
    - The annotation on secret `dns.gardener.cloud/provider` will mention the DNS provider name.
    - Secret data will give you the credentials for accessing the DNS provider account.
    - The annotation on secret `dns.gardener.cloud/domain` will provide the configured internal domain name.
- Now, go to the DNS provider console, and delete all the entries ending with `.<shoot-name>.<project-name>.shoot.<external-domain-name>`
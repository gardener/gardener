# Disaster Recovery: Restoring a Garden Cluster to a new Runtime Cluster 🛠️

This documentation outlines the procedure for restoring a **Garden cluster** into a new runtime cluster.
The primary goal is to minimize the impact on stakeholders and other components, particularly by preventing the invalidation of credentials issued before the restore.

## Disclaimer

> [!IMPORTANT]
> The restoration process described here assumes that no other actors (like `gardener-operator`, `etcd-druid`, DNS controller,  etc.) which also existed in the previous runtime cluster are active anymore.
> It is crucial to ensure that these components are scaled down or disabled, for example by invalidating their credentials to avoid conflicts.

## Required Backup Components (Building Blocks)

The restoration process requires specific building blocks — like `etcd` backup, credentials and configuration components — to be supplied, which necessitates their continuous backup.
The following sections provides details for these components.

### ETCD backup

The Garden cluster's ETCD is managed by `etcd-druid`, which can take care of performing continuous backups.
This can be configured in the `Garden` resource and is strongly recommended for any setup.

> [!IMPORTANT]
> Without ETCD backups, a restore is not possible.

Reference:
- [Backup and Restore Concept](https://github.com/gardener/gardener/blob/master/docs/concepts/backup-restore.md)
- [Garden ETCD](https://github.com/gardener/gardener/blob/master/docs/concepts/operator.md#etcd)
- [Example Backup Configuration](https://github.com/gardener/gardener/blob/1f9458b6eb73a8d1f489f003403e16d01bd014a9/example/operator/20-garden.yaml#L86-L93)

### `Garden` Resource

The `Garden` resource should be backed up in its entirety including the `status` subresource.
The `status` contains critical information like the current state of credentials rotation.

Failure to capture changes in the `status`, specifically for `.status.encryptedResources` and `.status.credentials` will lead to data loss.
Finding a reasonable backup frequency depends on the frequency of changes applied to the `Garden` and operations triggered.

### Runtime Data

To ensure a successful and less-disruptive restore, the following data containing state information must be backed up from the runtime cluster.
These are stored as `Secrets` in the **`garden` namespace** and their names are typically suffixed by content hashes and, if triggered, a hash indicating recent rotation (e.g., `ca-f6032ea0-5e58a`).

Preserving the encryption keys is absolutely critical, while the other secrets are necessary to avoid invalidating existing credentials.

#### 1. Encryption Keys and Configurations

These exist separately for both the `kube-apiserver` and the `gardener-apiserver`.
Without them, data stored in `etcd` cannot be decrypted, leading to complete data loss.

* `kube-apiserver-etcd-encryption-key`
* `kube-apiserver-etcd-encryption-configuration`
* `gardener-apiserver-etcd-encryption-key`
* `gardener-apiserver-etcd-encryption-configuration`

#### 2. Manually-Rotated Certificate Authorities (CAs)

The following CAs must be preserved:

* `ca`
* `ca-front-proxy`
* `ca-etcd`
* `ca-etcd-peer`
* `ca-gardener`
* `ca-client`

#### 3. Signing Keys

* `service-account-key`
* `gardener-apiserver-workload-identity-signing-key`

> [!NOTE]
> It is **not** necessary to store or restore secrets that have "bundle" in their name.

### Infrastructure Credentials

Typically, a `Garden` resources references infrastructure credentials for a DNS provider and the `etcd` backup bucket.
For a restore to succeed, these credentials must be valid and available in the new runtime cluster.

-----

## Restoration Procedure

The following steps detail the restoration process.

### Step 1: Shut down Actors in the previous Runtime Cluster

To avoid any conflicts caused by actors still running in the previous runtime cluster, ensure that all relevant components are scaled down or disabled.

If it is still possible, scale down the following `Deployments` in the `garden` namespace:
* `gardener-operator`
* `etcd-druid`
* `gardener-resource-manager`
* `gardener-apiserver`
* `virtual-garden-kube-apiserver`

If it is still possible, scale down the following `StatefulSets` in the `garden` namespace:
* `virtual-garden-etcd-main`
* `virtual-garden-etcd-events`

If it is still possible, scale down the following `Deployments` in the `runtime-extension-provider-<infrastructure>` namespace, where `<infrastructure>` is the provider extension used to manage the `DNSRecord` resource:
* `gardener-extension-provider-<infrastructure>`

There might be a case, where scaling down is not possible anymore. In this case, ensure that the credentials used by these components are invalidated before continuity with the restore procedure.

Specifically, invalidate the credentials referenced in the `garden` resource at:
* `.spec.dns.providers[].secretRef.name`
* `.spec.virtualCluster.etcd.main.backup.secretRef.name`

This will ensure that these components cannot update external resources anymore.

### Step 2: Create a Copy of ETCD Backups

In order to avoid data loss due to mistakes, it is strongly recommended to create a copy of the `etcd` backup.
This ensures that the restore procedure can be tried again with a pristine `etcd` backup in case of failure.

### Step 3: Create a new Runtime Cluster

Provision a new runtime cluster where the Garden cluster will be restored into.

### Step 4: Adjust CIDRs

In case the new runtime cluster has different CIDRs than the previous one, the `Garden` resource must be adjusted accordingly before applying it to the new cluster.

### Step 5: Deploy the `gardener-operator`

Deploy the `gardener-operator` into the new runtime cluster as described [here](../concepts/operator.md#deployment).
Ensure that it is the same version as the one used in the previous cluster to avoid compatibility issues.

### Step 6: Apply Backed-up Resources

1.  **Scale Down `gardener-operator`:** Scale down the `gardener-operator` deployment to prevent it from immediately reconciling a new `Garden` resource.
2.  **Delete Webhooks:** Delete the [mutating](../concepts/operator.md#defaulting) and [validating](../concepts/operator.md#validation) webhooks registered for the `Garden` resource to unblock later operations. Both webhooks are named `gardener-operator`. Once the operator is scaled up, it will recreate them.
3.  **Deploy State Secrets:** Deploy the correct backed-up secrets for CAs, encryption, and signing keys into the new cluster.
4.  **Deploy Infrastructure Secrets:** Deploy valid infrastructure credentials (e.g., for DNS and `etcd` backup) into the new cluster.
5.  **Apply `Garden` Resource:** Apply the backed-up **`Garden` resource** to the new cluster.

### Step 7: Configure Credential Rotation Status

This step is critical for `Garden` resources where credential rotation was in the **`Prepared`** phase.
This ensures the `gardener-operator` takes the correct code path and can complete the rotation later.

Patch the **status subresource** of the garden resource with content similar to the snippet below, reflecting the last completed rotation steps.

Patching can be done with `kubectl edit garden <garden-name> --subresource=status` or by applying a yaml file with `kubectl apply -f <file> --subresource=status`.

```yaml
observedGeneration: 1
credentials:
  rotation:
    certificateAuthorities:
      lastInitiationFinishedTime: "2025-06-27T10:43:01Z"
      lastInitiationTime: "2025-06-27T10:39:57Z"
      phase: Prepared # Must be set if rotation was in Prepared phase
    etcdEncryptionKey:
      lastInitiationFinishedTime: "2025-06-27T10:43:01Z"
      lastInitiationTime: "2025-06-27T10:39:57Z"
      phase: Prepared # Must be set if rotation was in Prepared phase
    observability:
      lastCompletionTime: "2025-06-27T10:43:01Z"
      lastInitiationTime: "2025-06-27T10:39:57Z"
    serviceAccountKey:
      lastInitiationFinishedTime: "2025-06-27T10:43:01Z"
      lastInitiationTime: "2025-06-27T10:39:57Z"
      phase: Prepared # Must be set if rotation was in Prepared phase
    workloadIdentityKey:
      lastInitiationFinishedTime: "2025-06-27T10:43:01Z"
      lastInitiationTime: "2025-06-27T10:39:57Z"
      phase: Prepared # Must be set if rotation was in Prepared phase
```

### Step 8: Configure Encrypted Resources

If the `Garden` specifies **additional resources for encryption** in the spec, this must be reflected in the status **before the first reconciliation**.
Failure to do so will make the `gardener-operator` assume that these resources were not yet encrypted.
As the consequence, it will attempt to encrypt them but the process will never finish, which will block the restore procedure.

Take the `.status.encryptedResources` section from the backed-up `Garden` resource and apply it.

In case the information is not present, it can be derived from the following fields in the spec:
    * `garden.spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig.resources`
    * `garden.spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources`

Patching can be done with `kubectl edit garden <garden-name> --subresource=status` or by applying a yaml file with `kubectl apply -f <file> --subresource=status`.

### Step 9: Start Restoration

**Scale up `gardener-operator`:** Scale the `gardener-operator` deployment back up.
It will now reconcile the `Garden` resource with the correct initial status.

### Step 10: Restoring multi-member ETCD clusters (HA configuration)

* For a Garden running with an **HA configuration**, restoring **the etcd cluster requires manual steps**.
* Follow the specific procedure documented in the `etcd-druid` repository:
    * **Reference:** `https://github.com/gardener/etcd-druid/blob/master/docs/usage/recovering-etcd-clusters.md`
* Apply the steps to the `etcd-main` cluster.

-----

## Edge Cases and Special Considerations

#### 1. Restoring from Credentials Rotation Phase `Prepared`

When restoring from the `Prepared` phase, the secrets deployed in Step 1 will have two suffixes: the content hash and a second suffix indicating a non-empty `last-rotation-initiation-time` label.

#### 2. Restoring from Credentials Rotation Phase `Preparing`

If the cluster is being restored from the `Preparing` phase (meaning rotation was actively in progress but not yet completed), be extra careful.
The encryption state of resources in **etcd** might require both the new and the old encryption keys to be available to the encryption configurations.

Whether this is the case or not depends on the time of the last successful backup of etcd data.

> [!NOTE]
> There are always two encryption configurations: one for the `kube-apiserver` and one for the `gardener-apiserver`.

See [Encryption At Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/) for more details.

#### 3. Encryption Keys

When performing a restore, ensure that the encryption keys and configurations deployed match the state that existed in the former runtime cluster.
Mismatched keys or an incorrect rotation status can cause the `gardener-operator` to issue new encryption keys.
This in turn will render existing data inaccessible.
Depending on the conditions, it might also cause data in `etcd` to be encrypted with new keys, leading to permanent data loss.

> [!TIP]
> Therefore, it is strongly recommended to create a copy of the etcd backups before starting the restore procedure.
> Having the ability to retry the restore procedure with a pristine etcd backup is crucial in case of mistakes.

## Testing Locally

Disaster Recovery can be tested with the [local development setup](../development/local_setup.md).
This may help to gain confidence before performing the procedure on production clusters.

### Preparation

For testing purposes, create a local `kind` cluster and deploy a Garden cluster into it by following the [Local Development Setup Guide](../deployment/getting_started_locally.md).
The recommended way is to use the `gardener-operator` and its respective `make` targets. At the time of writing this, the following commands can be used:

```console
make kind-multi-zone-up operator-up operator-seed-up
```

Once everything is up and running, create a couple of resources (`Project`, `Secret`, ...) in the virtual Garden cluster used to validate and restore procedure.

Depending on the testcases which should be covered, trigger a credentials rotation, configure an HA setup or add additional resources to be encrypted.

Finally, create a `ServiceAccount` in the virtual Garden cluster and have the API server issue a token for it.

```console
kubectl create token <service-account-name> -n <namespace> --duration=24h
```

Store the token into a [kubeconfig file](https://kubernetes.io/docs/reference/config-api/kubeconfig.v1/#AuthInfo) and test, that it can be used to access the virtual Garden cluster.
After a successful restore, this token should still be accepted.

### Persisting Backup and State Data

Create backups of the state and infrastructure credentials secrets in the `garden` namespace as described above.
Additionally, export the `Garden` resource including its status subresource.

To persist the `etcd` backups, create a copy of the local directory, where the `backup-restore` sidecar writes to.
For local development setups, this is `dev/local-backupbuckets/` and the directory is prefixed with `garden-`.
Note, that backups are created periodically, so wait some minutes to ensure that latest changes are included.

### Causing a Disaster

To simulate a disaster, simply delete the kind cluster:

```console
make kind-multi-zone-down
```

### Restoring the Garden Cluster

To start with the restore part, create a new kind cluster:

```console
make kind-multi-zone-up operator-up
```

Subsequently, copy the persisted etcd backups into the new local backup bucket directory at `dev/local-backupbuckets/`.

Before the `Garden` can be applied, the name of the backup bucket needs to be added to the `Garden` resource at `.spec.virtualCluster.etcd.main.backup.bucketName`.
The name matches the directory name of the local backup bucket created earlier (prefixed with `garden-`).

Now, follow the restoration procedure as described above, starting with scaling down the `gardener-operator`, deploying the secrets, applying the `Garden` resource and patching the status subresource, if necessary.

### Validation

The `Garden` resource should reconcile successfully and etcd should contain data from before the disaster.
Finally, use the token created before to validate that existing credentials are still valid after the restore.
